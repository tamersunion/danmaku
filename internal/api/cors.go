package api

import (
	"net/http"
	"net/url"
	"strings"
)

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		origins, credentials, allowHeaders := s.corsPolicy(r.URL.Path)
		if allowedOrigin(origin, origins) {
			value := origin
			if hasWildcard(origins) && !credentials {
				value = "*"
			}
			w.Header().Set("Access-Control-Allow-Origin", value)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			if credentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if allowHeaders {
				requested := r.Header.Get("Access-Control-Request-Headers")
				if requested == "" {
					requested = "Content-Type"
				}
				w.Header().Set("Access-Control-Allow-Headers", requested)
			}
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) corsPolicy(path string) ([]string, bool, bool) {
	switch {
	case strings.HasPrefix(path, "/api/live/danmaku"):
		return s.config.LiveWithOrigins, true, true
	case strings.HasPrefix(path, "/api/admin/"):
		return s.config.AdminWithOrigins, false, false
	case strings.HasPrefix(path, "/api/danmaku/") || strings.HasPrefix(path, "/api/other/"):
		return s.config.WithOrigins, false, true
	default:
		return nil, false, false
	}
}

func allowedOrigin(origin string, allowed []string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	for _, candidate := range allowed {
		if candidate == "*" || strings.EqualFold(strings.TrimSuffix(candidate, "/"), strings.TrimSuffix(origin, "/")) {
			return true
		}
		candidateURL, err := url.Parse(candidate)
		if err == nil && strings.HasPrefix(candidateURL.Hostname(), "*.") {
			suffix := strings.TrimPrefix(candidateURL.Hostname(), "*")
			if parsed.Scheme == candidateURL.Scheme && strings.HasSuffix(parsed.Hostname(), suffix) {
				return true
			}
		}
	}
	return false
}
