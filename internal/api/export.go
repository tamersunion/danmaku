package api

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"google.golang.org/protobuf/encoding/protowire"
)

type danuniJSON struct {
	SOID     string    `json:"SOID"`
	DMID     string    `json:"DMID"`
	Progress int64     `json:"progress"`
	Mode     int       `json:"mode"`
	FontSize int       `json:"fontsize"`
	Color    int       `json:"color"`
	SenderID string    `json:"senderID"`
	Content  string    `json:"content"`
	CTime    time.Time `json:"ctime"`
	Weight   int       `json:"weight"`
	Pool     int       `json:"pool"`
	Attr     []string  `json:"attr"`
	Platform string    `json:"platform"`
}

func (s *Server) exportDanmaku(w http.ResponseWriter, r *http.Request) {
	vid := strings.TrimSpace(r.URL.Query().Get("id"))
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if vid == "" || format == "" {
		s.writeJSON(w, http.StatusBadRequest, result{Code: 1, Data: map[string]string{"desc": "id 和 format 不能为空"}})
		return
	}
	if !supportedExportFormat(format) {
		s.writeJSON(w, http.StatusBadRequest, result{Code: 1, Data: map[string]string{"desc": "不支持的输出格式"}})
		return
	}
	data, err := s.queryDanmakuByVID(r, vid)
	if err != nil {
		s.writeError(w, err)
		return
	}
	data = offsetDanmaku(data, queryFloat(r.URL.Query().Get("offset"), 0))
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(vid+exportExtension(format)))
	switch format {
	case "common", "common.json":
		s.writeJSON(w, http.StatusOK, success(data))
	case "dplayer", "dplayer.json":
		s.writeJSON(w, http.StatusOK, success(exportDPlayerRows(data)))
	case "artplayer", "artplayer.json":
		s.writeJSON(w, http.StatusOK, map[string]any{"danmuku": exportArtPlayerData(data)})
	case "bilibili", "bilibili.xml":
		s.writeXML(w, data)
	case "danuni", "danuni.json":
		s.writeJSON(w, http.StatusOK, danuniData(vid, data))
	case "danuni.pb":
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(danuniProtobuf(vid, data))
	case "ddplay", "ddplay.json":
		s.writeJSON(w, http.StatusOK, ddplayData(data))
	case "vod", "vod.json":
		s.writeJSON(w, http.StatusOK, vodData(vid, data))
	case "baha", "baha.json":
		s.writeJSON(w, http.StatusOK, bahaData(data))
	case "ass":
		w.Header().Set("Content-Type", "text/x-ssa; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(assData(vid, data)))
	}
}

func supportedExportFormat(format string) bool {
	switch format {
	case "common", "common.json", "dplayer", "dplayer.json", "artplayer", "artplayer.json",
		"bilibili", "bilibili.xml", "danuni", "danuni.json", "danuni.pb", "ddplay", "ddplay.json",
		"vod", "vod.json", "baha", "baha.json", "ass":
		return true
	default:
		return false
	}
}

func exportExtension(format string) string {
	switch format {
	case "bilibili", "bilibili.xml":
		return ".xml"
	case "danuni.pb":
		return ".danuni.pb"
	case "ass":
		return ".ass"
	default:
		return ".json"
	}
}

func danuniData(vid string, data []domain.DanmakuData) []danuniJSON {
	result := make([]danuniJSON, 0, len(data))
	for index, item := range data {
		content := pointerValue(item.Text)
		sender := strings.TrimSpace(item.Author)
		if sender == "" && item.AuthorID != 0 {
			sender = strconv.Itoa(item.AuthorID)
		}
		if sender == "" {
			sender = "anonymous"
		}
		result = append(result, danuniJSON{
			SOID: "def_" + vid + "@danmaku.local", DMID: exportDMID(vid, index, item),
			Progress: int64(math.Round(float64(item.Time) * 1000)), Mode: danuniMode(item.Mode), FontSize: item.Size,
			Color: item.Color, SenderID: sender + "@danmaku.local", Content: content,
			CTime: time.Unix(item.Timestamp, 0).UTC(), Pool: max(0, min(item.Pool, 3)), Attr: []string{}, Platform: "danmaku.local",
		})
	}
	return result
}

func exportDPlayerRows(data []domain.DanmakuData) [][]any {
	rows := make([][]any, 0, len(data))
	for _, item := range data {
		typeValue, text := playerValues(item)
		author := item.Author
		if author == "" && item.AuthorID != 0 {
			author = strconv.Itoa(item.AuthorID)
		}
		rows = append(rows, []any{item.Time, typeValue, item.Color, author, pointerValue(text)})
	}
	return rows
}

func exportArtPlayerData(data []domain.DanmakuData) []domain.ArtPlayerData {
	result := make([]domain.ArtPlayerData, 0, len(data))
	for _, item := range data {
		mode, text := playerValues(item)
		if item.Mode == 4 || item.Mode == 5 {
			mode = 1
		}
		result = append(result, domain.ArtPlayerData{Text: text, Time: item.Time, Color: exportColor(item.Color), Size: item.Size, Border: false, Mode: mode})
	}
	return result
}

func exportColor(color int) string {
	return fmt.Sprintf("#%06X", color&0xffffff)
}

func danuniMode(mode int) int {
	switch mode {
	case 4:
		return 1
	case 5:
		return 2
	case 6:
		return 3
	case 7, 8, 9:
		return 4
	default:
		return 0
	}
}

func exportDMID(vid string, index int, item domain.DanmakuData) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%g\x00%d\x00%s", vid, index, item.Time, item.Timestamp, pointerValue(item.Text))))
	return hex.EncodeToString(hash[:16]) + "@danmaku.local"
}

func danuniProtobuf(vid string, data []domain.DanmakuData) []byte {
	var response []byte
	for _, item := range danuniData(vid, data) {
		var message []byte
		message = appendProtoString(message, 1, item.SOID)
		message = appendProtoString(message, 2, item.DMID)
		message = protowire.AppendTag(message, 3, protowire.VarintType)
		message = protowire.AppendVarint(message, uint64(item.Progress))
		message = appendProtoVarint(message, 4, uint64(item.Mode))
		message = appendProtoVarint(message, 5, uint64(item.FontSize))
		message = appendProtoVarint(message, 6, uint64(item.Color))
		message = appendProtoString(message, 7, item.SenderID)
		message = appendProtoString(message, 8, item.Content)
		var timestamp []byte
		timestamp = appendProtoVarint(timestamp, 1, uint64(item.CTime.Unix()))
		message = appendProtoBytes(message, 9, timestamp)
		message = appendProtoVarint(message, 10, uint64(item.Weight))
		message = appendProtoVarint(message, 11, uint64(item.Pool))
		message = appendProtoString(message, 13, item.Platform)
		response = appendProtoBytes(response, 1, message)
	}
	return response
}

func appendProtoString(target []byte, field protowire.Number, value string) []byte {
	return appendProtoBytes(target, field, []byte(value))
}

func appendProtoBytes(target []byte, field protowire.Number, value []byte) []byte {
	target = protowire.AppendTag(target, field, protowire.BytesType)
	return protowire.AppendBytes(target, value)
}

func appendProtoVarint(target []byte, field protowire.Number, value uint64) []byte {
	target = protowire.AppendTag(target, field, protowire.VarintType)
	return protowire.AppendVarint(target, value)
}

func ddplayData(data []domain.DanmakuData) map[string]any {
	comments := make([]map[string]any, 0, len(data))
	for index, item := range data {
		mode := item.Mode
		if mode == 0 {
			mode = 1
		}
		comments = append(comments, map[string]any{
			"cid": index + 1,
			"p":   fmt.Sprintf("%g,%d,%d,%s", item.Time, mode, item.Color, item.Author),
			"m":   pointerValue(item.Text),
		})
	}
	return map[string]any{"count": len(comments), "comments": comments}
}

func vodData(vid string, data []domain.DanmakuData) map[string]any {
	rows := make([][]any, 0, len(data))
	for _, item := range data {
		position := "right"
		if item.Mode == 4 {
			position = "bottom"
		} else if item.Mode == 5 {
			position = "top"
		}
		rows = append(rows, []any{item.Time, position, exportColor(item.Color), "", pointerValue(item.Text), "", "", strconv.Itoa(item.Size) + "px"})
	}
	return map[string]any{"code": 0, "name": vid, "danum": len(rows), "danmuku": rows}
}

func bahaData(data []domain.DanmakuData) map[string]any {
	rows := make([]map[string]any, 0, len(data))
	for index, item := range data {
		position := 0
		if item.Mode == 5 {
			position = 1
		} else if item.Mode == 4 {
			position = 2
		}
		size := 1
		if item.Size <= 18 {
			size = 0
		} else if item.Size > 25 {
			size = 2
		}
		rows = append(rows, map[string]any{"sn": index + 1, "text": pointerValue(item.Text), "time": int(math.Round(float64(item.Time) * 10)), "color": exportColor(item.Color), "size": size, "position": position, "userid": item.Author})
	}
	return map[string]any{"data": map[string]any{"danmu": rows, "totalCount": len(rows)}}
}

func assData(vid string, data []domain.DanmakuData) string {
	var builder strings.Builder
	builder.WriteString("[Script Info]\nTitle: ")
	builder.WriteString(strings.ReplaceAll(vid, "\n", " "))
	builder.WriteString("\nScriptType: v4.00+\nPlayResX: 1920\nPlayResY: 1080\n")
	if raw := gzipBase64(danuniProtobuf(vid, data)); raw != "" {
		builder.WriteString(";RawCompressType: gzip\n;RawBaseType: base64\n;RawPb: ")
		builder.WriteString(raw)
		builder.WriteByte('\n')
	}
	builder.WriteString("\n[V4+ Styles]\n")
	builder.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	builder.WriteString("Style: Danmaku,Arial,25,&H00FFFFFF,&H00FFFFFF,&H80000000,&H80000000,0,0,0,0,100,100,0,0,1,1,0,7,0,0,0,1\n\n[Events]\n")
	builder.WriteString("Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	for _, item := range data {
		start := max(float64(item.Time), 0)
		end := start + 8
		alignment := ""
		if item.Mode == 5 {
			alignment = "{\\an8}"
		} else if item.Mode == 4 {
			alignment = "{\\an2}"
		}
		color := fmt.Sprintf("{\\c&H%02X%02X%02X&\\fs%d}", item.Color&0xff, item.Color>>8&0xff, item.Color>>16&0xff, item.Size)
		text := strings.NewReplacer("\\", "\\\\", "{", "\\{", "}", "\\}", "\r", "", "\n", "\\N").Replace(pointerValue(item.Text))
		builder.WriteString(fmt.Sprintf("Dialogue: 0,%s,%s,Danmaku,,0,0,0,,%s%s%s\n", assTime(start), assTime(end), alignment, color, text))
	}
	return builder.String()
}

func gzipBase64(raw []byte) string {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(raw); err != nil {
		return ""
	}
	if err := writer.Close(); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(compressed.Bytes())
}

func assTime(seconds float64) string {
	hundredths := int64(math.Round(seconds * 100))
	hours := hundredths / 360000
	hundredths %= 360000
	minutes := hundredths / 6000
	hundredths %= 6000
	return fmt.Sprintf("%d:%02d:%02d.%02d", hours, minutes, hundredths/100, hundredths%100)
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
