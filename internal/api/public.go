package api

import (
	"encoding/xml"
	"errors"
	"html"
	"net"
	"net/http"
	"strconv"
	"strings"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"git.hanada.info/tamersunion/danmaku/internal/store"
)

func (s *Server) serveDPlayer(w http.ResponseWriter, r *http.Request, path string) {
	const base = "/api/danmaku/dplayer/v3"
	switch {
	case path == base+"/iqiyi" && r.Method == http.MethodGet:
		vid := foldQuery(r, "vid")
		if vid == "" {
			http.NotFound(w, r)
			return
		}
		data, err := s.iqiyi.Data(r.Context(), vid)
		if err != nil {
			s.writeError(w, err)
			return
		}
		s.writeJSON(w, http.StatusOK, success(iqiyiDPlayerRows(data)))
	case path == base+"/bilibili" && r.Method == http.MethodGet:
		data, err := s.bilibili.Data(r.Context(), bilibiliQueryFromRequest(r))
		if err != nil {
			s.writeError(w, err)
			return
		}
		s.writeJSON(w, http.StatusOK, success(dplayerRows(data)))
	case path == base && r.Method == http.MethodGet:
		id := r.URL.Query().Get("id")
		if id == "" {
			s.writeJSON(w, http.StatusOK, failure())
			return
		}
		data, err := s.queryDanmakuByVID(r, id)
		if err != nil {
			s.writeError(w, err)
			return
		}
		s.writeJSON(w, http.StatusOK, success(dplayerRows(data)))
	case path == base && r.Method == http.MethodPost:
		input := domain.DPlayerInput{}
		if !s.decodeJSON(w, r, &input) {
			return
		}
		if strings.TrimSpace(input.ID) == "" || strings.TrimSpace(input.Text) == "" {
			s.writeJSON(w, http.StatusOK, failure())
			return
		}
		data := domain.NewDanmakuData()
		data.Time, data.Color, data.Text = input.Time, input.Color, stringPointer(input.Text)
		switch input.Type {
		case 1:
			data.Mode = 5
		case 2:
			data.Mode = 4
		default:
			data.Mode = 1
		}
		if authorID, err := strconv.Atoi(input.Author); err == nil {
			data.AuthorID = authorID
		} else {
			data.Author = input.Author
		}
		s.insertDanmaku(w, r, input.ID, data, input.IP, input.Referer)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) serveCommon(w http.ResponseWriter, r *http.Request, path string) {
	const base = "/api/danmaku/v1"
	if strings.HasPrefix(path, base+"/bilibili") {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		s.serveBilibiliData(w, r, path, base+"/bilibili", false)
		return
	}
	if r.Method == http.MethodPost && path == base {
		input := domain.DanmakuInput{DanmakuData: domain.NewDanmakuData()}
		if !s.decodeJSON(w, r, &input) {
			return
		}
		if strings.TrimSpace(input.ID) == "" || input.Text == nil || strings.TrimSpace(*input.Text) == "" {
			s.writeJSON(w, http.StatusOK, failure())
			return
		}
		s.insertDanmaku(w, r, input.ID, input.DanmakuData, input.IP, input.Referer)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id, format := routeIDAndFormat(path, base)
	if id == "" {
		id = r.URL.Query().Get("id")
	}
	if id == "" {
		s.writeJSON(w, http.StatusOK, failure())
		return
	}
	data, err := s.queryDanmakuByVID(r, id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if strings.EqualFold(format, "xml") {
		s.writeXML(w, data)
		return
	}
	s.writeJSON(w, http.StatusOK, success(data))
}

func (s *Server) serveArtPlayer(w http.ResponseWriter, r *http.Request, path string) {
	const base = "/api/danmaku/artplayer/v1"
	if strings.HasPrefix(path, base+"/bilibili") {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		s.serveBilibiliData(w, r, path, base+"/bilibili", true)
		return
	}
	if r.Method == http.MethodPost && path == base {
		input := domain.ArtPlayerInput{Size: 25}
		if !s.decodeJSON(w, r, &input) {
			return
		}
		if strings.TrimSpace(input.ID) == "" || strings.TrimSpace(input.Text) == "" {
			s.writeJSON(w, http.StatusOK, failure())
			return
		}
		color, err := strconv.ParseInt(strings.TrimPrefix(input.Color, "#"), 16, 32)
		if err != nil {
			s.writeJSON(w, http.StatusOK, failure())
			return
		}
		data := domain.NewDanmakuData()
		data.Time, data.Mode, data.Color, data.Size, data.Text = input.Time, input.Mode, int(color), input.Size, stringPointer(input.Text)
		s.insertDanmaku(w, r, input.ID, data, input.IP, input.Referer)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id, format := routeIDAndFormat(path, base)
	if id == "" {
		id = r.URL.Query().Get("id")
	}
	if id == "" {
		s.writeJSON(w, http.StatusOK, failure())
		return
	}
	data, err := s.queryDanmakuByVID(r, id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if strings.EqualFold(format, "json") {
		s.writeJSON(w, http.StatusOK, success(artPlayerData(data)))
		return
	}
	s.writeXML(w, data)
}

func (s *Server) serveBilibiliData(w http.ResponseWriter, r *http.Request, path, base string, artPlayer bool) {
	format := bilibiliFormat(path, base)
	query := bilibiliQueryFromRequest(r)
	data, err := s.bilibili.Data(r.Context(), query)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if strings.EqualFold(format, "json") {
		if artPlayer {
			s.writeJSON(w, http.StatusOK, success(artPlayerData(data)))
		} else {
			s.writeJSON(w, http.StatusOK, success(data))
		}
		return
	}
	s.writeXML(w, data)
}

func (s *Server) insertDanmaku(w http.ResponseWriter, r *http.Request, vid string, data domain.DanmakuData, inputIP net.IP, inputReferer string) {
	ip := requestIP(r, inputIP)
	refererRaw := inputReferer
	if refererRaw == "" {
		refererRaw = r.Header.Get("Referer")
	}
	referer, err := store.ParseReferer(refererRaw)
	if err != nil {
		s.writeJSON(w, http.StatusOK, failure())
		return
	}
	if _, err := s.repository.Insert(r.Context(), vid, data, ip, referer); errors.Is(err, store.ErrVideoDeleted) {
		s.writeJSON(w, http.StatusOK, failure())
		return
	} else if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, success(nil))
}

func (s *Server) queryDanmakuByVID(r *http.Request, vid string) ([]domain.DanmakuData, error) {
	video, err := s.repository.EnsureVideo(r.Context(), vid)
	if err != nil {
		return nil, err
	}
	if video == nil || video.IsDeleted {
		return []domain.DanmakuData{}, nil
	}
	local, err := s.repository.QueryByVid(r.Context(), vid)
	if err != nil {
		return nil, err
	}
	linked, err := s.bilibili.BoundData(r.Context(), vid)
	if err != nil {
		return nil, err
	}
	return offsetDanmaku(append(local, linked...), 0), nil
}

func requestIP(r *http.Request, fallback net.IP) net.IP {
	if parsed := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); parsed != nil {
		return parsed
	}
	if forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ","); len(forwarded) > 0 {
		if parsed := net.ParseIP(strings.TrimSpace(forwarded[0])); parsed != nil {
			return parsed
		}
	}
	if fallback != nil {
		return fallback
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return net.ParseIP(host)
	}
	return net.ParseIP(r.RemoteAddr)
}

func routeIDAndFormat(path, base string) (string, string) {
	suffix := strings.TrimPrefix(path, base)
	suffix = strings.TrimPrefix(suffix, "/")
	if suffix == "" {
		return "", ""
	}
	if index := strings.LastIndex(suffix, "."); index >= 0 {
		return suffix[:index], suffix[index+1:]
	}
	return suffix, ""
}

func bilibiliFormat(path, base string) string {
	suffix := strings.TrimPrefix(path, base)
	suffix = strings.TrimPrefix(suffix, "/")
	if strings.HasPrefix(suffix, "danmaku.") {
		return strings.TrimPrefix(suffix, "danmaku.")
	}
	return ""
}

func dplayerRows(data []domain.DanmakuData) [][]any {
	rows := make([][]any, 0, len(data))
	for _, item := range data {
		typeValue, text := playerValues(item)
		author := item.Author
		if author == "" {
			author = strconv.Itoa(item.AuthorID)
		}
		var escapedText any
		if text != nil {
			escapedText = html.EscapeString(*text)
		}
		rows = append(rows, []any{item.Time, typeValue, item.Color, html.EscapeString(author), escapedText})
	}
	return rows
}

func artPlayerData(data []domain.DanmakuData) []domain.ArtPlayerData {
	result := make([]domain.ArtPlayerData, 0, len(data))
	for _, item := range data {
		mode, text := playerValues(item)
		if item.Mode == 4 || item.Mode == 5 {
			mode = 1
		}
		result = append(result, domain.ArtPlayerData{Text: text, Time: item.Time, Color: fmtColor(item.Color), Size: item.Size, Border: false, Mode: mode})
	}
	return result
}

func playerValues(item domain.DanmakuData) (int, *string) {
	text := item.Text
	switch item.Mode {
	case 4:
		return 2, text
	case 5:
		return 1, text
	case 7:
		if text != nil {
			parts := strings.Split(*text, ",")
			if len(parts) > 4 {
				text = stringPointer(parts[4])
			}
		}
		return 0, text
	case 8:
		return 0, nil
	default:
		return 0, text
	}
}

func fmtColor(color int) string { return "#" + strings.ToUpper(strconv.FormatInt(int64(color), 16)) }

func stringPointer(value string) *string { return &value }

type xmlDocument struct {
	XMLName xml.Name   `xml:"i"`
	Items   []xmlEntry `xml:"d"`
}

type xmlEntry struct {
	Parameters string `xml:"p,attr"`
	Text       string `xml:",chardata"`
}

func (s *Server) writeXML(w http.ResponseWriter, data []domain.DanmakuData) {
	document := xmlDocument{Items: make([]xmlEntry, 0, len(data))}
	for _, item := range data {
		text := ""
		if item.Text != nil {
			text = *item.Text
		}
		parameters := strings.Join([]string{
			strconv.FormatFloat(float64(item.Time), 'f', -1, 32), strconv.Itoa(item.Mode), strconv.Itoa(item.Size),
			strconv.Itoa(item.Color), strconv.FormatInt(item.Timestamp, 10), strconv.Itoa(item.Pool), item.Author,
			strconv.FormatInt(item.Timestamp, 10),
		}, ",")
		document.Items = append(document.Items, xmlEntry{Parameters: parameters, Text: text})
	}
	raw, err := xml.Marshal(document)
	if err != nil {
		s.writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(raw)
}
