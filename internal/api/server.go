package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	casclient "git.hanada.info/tamersunion/danmaku/internal/cas"
	"git.hanada.info/tamersunion/danmaku/internal/config"
	"git.hanada.info/tamersunion/danmaku/internal/store"
	"github.com/philippseith/signalr"
)

type Server struct {
	config        config.Config
	repository    store.Repository
	bilibili      *Bilibili
	iqiyi         *Iqiyi
	cas           *casclient.Client
	sessionSecret []byte
	logger        *slog.Logger
	mux           *http.ServeMux
	staticDir     string
}

func New(ctx context.Context, cfg config.Config, repository store.Repository, logger *slog.Logger) (*Server, error) {
	if logger == nil {
		logger = slog.Default()
	}
	secret := sha256.Sum256([]byte(cfg.Admin.Password + "\x00" + cfg.DanmakuSQL.Password + "\x00" + cfg.DanmakuSQL.Database))
	server := &Server{
		config: cfg, repository: repository, logger: logger,
		sessionSecret: secret[:], mux: http.NewServeMux(),
		staticDir: discoverStaticDir(),
	}
	server.bilibili = NewBilibili(repository, cfg.Bilibili)
	server.iqiyi = NewIqiyi(repository, cfg.Iqiyi)
	if cfg.CAS.Enabled {
		client, err := casclient.NewClient(cfg.CAS.BaseURL, cfg.CAS.ValidationURL, cfg.CAS.ValidationHost, time.Duration(cfg.CAS.RequestTimeoutSeconds)*time.Second)
		if err != nil {
			return nil, fmt.Errorf("configure CAS client: %w", err)
		}
		server.cas = client
	}
	if err := server.mapRealtime(ctx); err != nil {
		return nil, err
	}
	server.mux.HandleFunc("/api/", server.serveAPI)
	server.mux.HandleFunc("/cas/", server.serveCAS)
	server.mux.HandleFunc("/", server.serveSPA)
	return server, nil
}

func (s *Server) Handler() http.Handler {
	return s.recoverPanic(s.logRequests(s.cors(s.mux)))
}

func (s *Server) serveAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch {
	case path == "/api/admin/auth/options" && r.Method == http.MethodGet:
		s.authOptions(w, r)
	case path == "/api/admin/session" && r.Method == http.MethodGet:
		s.currentSession(w, r)
	case (path == "/api/admin/login" || path == "/api/admin/noAuth") && r.Method == http.MethodGet:
		s.writeJSON(w, http.StatusOK, result{Code: 401, Data: map[string]string{"desc": "没有权限"}})
	case path == "/api/admin/login" && r.Method == http.MethodPost:
		s.login(w, r)
	case path == "/api/admin/logout" && r.Method == http.MethodGet:
		s.logout(w, r)
	case strings.HasPrefix(path, "/api/admin/"):
		value, ok := s.requestSession(r)
		if !ok {
			s.writeJSON(w, http.StatusOK, result{Code: 401, Data: map[string]string{"desc": "没有权限"}})
			return
		}
		s.serveAdmin(w, r, path, value)
	case path == "/api/other/bilibili/queryaid" && r.Method == http.MethodGet:
		s.queryAID(w, r)
	case path == "/api/danmaku/export" && r.Method == http.MethodGet:
		s.exportDanmaku(w, r)
	case strings.HasPrefix(path, "/api/danmaku/dplayer/v3"):
		s.serveDPlayer(w, r, path)
	case strings.HasPrefix(path, "/api/danmaku/artplayer/v1"):
		s.serveArtPlayer(w, r, path)
	case strings.HasPrefix(path, "/api/danmaku/v1"):
		s.serveCommon(w, r, path)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) serveSPA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.staticDir == "" {
		http.NotFound(w, r)
		return
	}
	cleaned := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if cleaned == "." {
		cleaned = "index.html"
	}
	target := filepath.Join(s.staticDir, cleaned)
	root, rootErr := filepath.Abs(s.staticDir)
	resolved, targetErr := filepath.Abs(target)
	if rootErr != nil || targetErr != nil || (resolved != root && !strings.HasPrefix(resolved, root+string(filepath.Separator))) {
		http.NotFound(w, r)
		return
	}
	if info, err := os.Stat(target); err != nil || info.IsDir() {
		target = filepath.Join(s.staticDir, "index.html")
	}
	http.ServeFile(w, r, target)
}

func discoverStaticDir() string {
	candidates := []string{"wwwroot", filepath.Join("frontend", "dist")}
	for _, candidate := range candidates {
		if info, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		s.logger.Error("write JSON response", "error", err)
	}
}

func (s *Server) writeError(w http.ResponseWriter, err error) {
	s.logger.Error("request failed", "error", err)
	s.writeJSON(w, http.StatusInternalServerError, result{Code: 1})
}

func (s *Server) decodeJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	return s.decodeJSONLimit(w, r, value, 1<<20)
}

func (s *Server) decodeJSONLimit(w http.ResponseWriter, r *http.Request, value any, limit int64) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	if err := decoder.Decode(value); err != nil {
		s.writeJSON(w, http.StatusBadRequest, result{Code: 1})
		return false
	}
	return true
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Debug("request", "method", r.Method, "path", r.URL.Path, "elapsed", time.Since(started))
	})
}

func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Error("request panic", "value", recovered, "method", r.Method, "path", r.URL.Path)
				s.writeJSON(w, http.StatusInternalServerError, result{Code: 1})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) mapRealtime(ctx context.Context) error {
	registry := newGroupRegistry()
	hubServer, err := signalr.NewServer(ctx,
		signalr.HubFactory(func() signalr.HubInterface { return &liveHub{registry: registry} }),
		signalr.KeepAliveInterval(15*time.Second),
		signalr.InsecureSkipVerify(hasWildcard(s.config.LiveWithOrigins)),
		signalr.AllowOriginPatterns(originHosts(s.config.LiveWithOrigins)),
	)
	if err != nil {
		return fmt.Errorf("create SignalR server: %w", err)
	}
	hubServer.MapHTTP(signalr.WithHTTPServeMux(s.mux), "/api/live/danmaku")
	return nil
}

func hasWildcard(origins []string) bool {
	for _, origin := range origins {
		if origin == "*" {
			return true
		}
	}
	return false
}

func originHosts(origins []string) []string {
	hosts := make([]string, 0, len(origins))
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin == "" || origin == "*" {
			continue
		}
		origin = strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://")
		hosts = append(hosts, origin)
	}
	return hosts
}
