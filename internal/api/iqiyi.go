package api

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"github.com/andybalholm/brotli"
)

const (
	defaultIqiyiDecodeAPIBase    = "https://pcw-api.iq.com/api/decode"
	defaultIqiyiVideoInfoAPIBase = "https://pcw-api.iqiyi.com/video/video/baseinfo"
	defaultIqiyiDanmakuAPIBase   = "https://cmts.iqiyi.com/bullet"
	iqiyiSegmentSeconds          = 60
	iqiyiSegmentConcurrency      = 8
	iqiyiUserAgent               = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	maxIqiyiVideoSeconds         = 24 * 60 * 60
	maxIqiyiMetadataBytes        = 1 << 20
	maxIqiyiSegmentBytes         = 8 << 20
	maxIqiyiPayloadBytes         = 32 << 20
)

type Iqiyi struct {
	client           *http.Client
	decodeAPIBase    string
	videoInfoAPIBase string
	danmakuAPIBase   string
}

type iqiyiVideoInfo struct {
	Duration       float64
	DisplayDanmaku bool
}

type iqiyiProtoField struct {
	number int
	value  string
	bytes  []byte
}

func NewIqiyi() *Iqiyi {
	return &Iqiyi{
		client:           &http.Client{Timeout: 60 * time.Second},
		decodeAPIBase:    defaultIqiyiDecodeAPIBase,
		videoInfoAPIBase: defaultIqiyiVideoInfoAPIBase,
		danmakuAPIBase:   defaultIqiyiDanmakuAPIBase,
	}
}

func (i *Iqiyi) Data(ctx context.Context, vid string) ([]domain.DanmakuData, error) {
	vid = strings.TrimSpace(vid)
	if vid == "" {
		return []domain.DanmakuData{}, nil
	}

	decodedVID, err := i.decodeVID(ctx, vid)
	if err != nil {
		return nil, err
	}
	if decodedVID == "" {
		return []domain.DanmakuData{}, nil
	}

	selectedVID := decodedVID
	info, infoErr := i.videoInfo(ctx, decodedVID)
	if (infoErr != nil || info.Duration <= 0) && decodedVID != vid {
		fallback, fallbackErr := i.videoInfo(ctx, vid)
		if fallbackErr == nil && fallback.Duration > 0 {
			selectedVID, info, infoErr = vid, fallback, nil
		} else if infoErr == nil {
			infoErr = fallbackErr
		}
	}
	if infoErr != nil {
		return nil, infoErr
	}
	if info.Duration <= 0 || !info.DisplayDanmaku {
		return []domain.DanmakuData{}, nil
	}
	if info.Duration > maxIqiyiVideoSeconds {
		return nil, fmt.Errorf("iqiyi video duration %.0f exceeds the supported limit", info.Duration)
	}

	segmentCount := int(math.Ceil(info.Duration / iqiyiSegmentSeconds))
	return i.fetchSegments(ctx, selectedVID, segmentCount)
}

func (i *Iqiyi) decodeVID(ctx context.Context, vid string) (string, error) {
	endpoint := strings.TrimRight(i.decodeAPIBase, "/") + "/" + url.PathEscape(vid) + "?platformId=3&modeCode=intl&langCode=sg"
	raw, err := i.fetch(ctx, endpoint, maxIqiyiMetadataBytes, false)
	if err != nil {
		return "", fmt.Errorf("decode iqiyi vid: %w", err)
	}
	var response struct {
		Code string          `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return "", fmt.Errorf("decode iqiyi vid response: %w", err)
	}
	if response.Code != "0" {
		return "", nil
	}
	return jsonScalarString(response.Data), nil
}

func (i *Iqiyi) videoInfo(ctx context.Context, vid string) (iqiyiVideoInfo, error) {
	endpoint := strings.TrimRight(i.videoInfoAPIBase, "/") + "/" + url.PathEscape(vid)
	raw, err := i.fetch(ctx, endpoint, maxIqiyiMetadataBytes, false)
	if err != nil {
		return iqiyiVideoInfo{}, fmt.Errorf("fetch iqiyi video info: %w", err)
	}
	var response struct {
		Code string          `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return iqiyiVideoInfo{}, fmt.Errorf("decode iqiyi video info response: %w", err)
	}
	if response.Code != "A00000" {
		return iqiyiVideoInfo{}, nil
	}
	var data struct {
		Duration       json.RawMessage `json:"durationSec"`
		DisplayDanmaku *bool           `json:"displayBarrage"`
	}
	if err := json.Unmarshal(response.Data, &data); err != nil {
		return iqiyiVideoInfo{}, fmt.Errorf("decode iqiyi video info data: %w", err)
	}
	info := iqiyiVideoInfo{Duration: jsonScalarFloat(data.Duration), DisplayDanmaku: true}
	if data.DisplayDanmaku != nil {
		info.DisplayDanmaku = *data.DisplayDanmaku
	}
	return info, nil
}

func (i *Iqiyi) fetchSegments(ctx context.Context, vid string, count int) ([]domain.DanmakuData, error) {
	if count <= 0 {
		return []domain.DanmakuData{}, nil
	}
	results := make([][]domain.DanmakuData, count)
	jobs := make(chan int)
	workers := min(count, iqiyiSegmentConcurrency)
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for page := range jobs {
				data, err := i.fetchSegment(ctx, vid, page)
				if err == nil {
					results[page-1] = data
				}
			}
		}()
	}
	for page := 1; page <= count; page++ {
		select {
		case jobs <- page:
		case <-ctx.Done():
			close(jobs)
			wait.Wait()
			return nil, ctx.Err()
		}
	}
	close(jobs)
	wait.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	combined := make([]domain.DanmakuData, 0)
	for _, result := range results {
		combined = append(combined, result...)
	}
	return combined, nil
}

func (i *Iqiyi) fetchSegment(ctx context.Context, vid string, page int) ([]domain.DanmakuData, error) {
	endpoint := i.segmentURL(vid, page)
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		raw, err := i.fetch(ctx, endpoint, maxIqiyiSegmentBytes, true)
		if errors.Is(err, errIqiyiSegmentNotFound) {
			return []domain.DanmakuData{}, nil
		}
		if err != nil {
			lastErr = err
			continue
		}
		payload, err := readLimited(brotli.NewReader(bytes.NewReader(raw)), maxIqiyiPayloadBytes)
		if err != nil {
			lastErr = fmt.Errorf("decompress iqiyi segment: %w", err)
			continue
		}
		if len(payload) == 0 {
			return []domain.DanmakuData{}, nil
		}
		if payload[0] == '<' {
			return parseIqiyiXML(payload)
		}
		return parseIqiyiProto(payload)
	}
	return nil, lastErr
}

func (i *Iqiyi) segmentURL(vid string, page int) string {
	padded := "0000" + vid
	path := padded[len(padded)-4:len(padded)-2] + "/" + padded[len(padded)-2:]
	source := fmt.Sprintf("%s_%d_%dcbzuw1259a", vid, iqiyiSegmentSeconds, page)
	sum := md5.Sum([]byte(source))
	signature := hex.EncodeToString(sum[:])
	signature = signature[len(signature)-8:]
	return fmt.Sprintf("%s/%s/%s_%d_%d_%s.br", strings.TrimRight(i.danmakuAPIBase, "/"), path, url.PathEscape(vid), iqiyiSegmentSeconds, page, signature)
}

var errIqiyiSegmentNotFound = errors.New("iqiyi segment not found")

func (i *Iqiyi) fetch(ctx context.Context, endpoint string, limit int64, segment bool) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", iqiyiUserAgent)
	if segment {
		request.Header.Set("Accept-Encoding", "br")
		request.Header.Set("Content-Type", "application/octet-stream")
	} else {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := i.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if segment && response.StatusCode == http.StatusNotFound {
		return nil, errIqiyiSegmentNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("iqiyi upstream returned HTTP %d", response.StatusCode)
	}
	return readLimited(response.Body, limit)
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return raw, nil
}

func parseIqiyiXML(raw []byte) ([]domain.DanmakuData, error) {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	contents := make([]string, 0)
	times := make([]string, 0)
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || (start.Name.Local != "content" && start.Name.Local != "showTime") {
			continue
		}
		var value string
		if err := decoder.DecodeElement(&value, &start); err != nil {
			return nil, err
		}
		if start.Name.Local == "content" {
			contents = append(contents, value)
		} else {
			times = append(times, value)
		}
	}
	return iqiyiDanmaku(contents, times, false), nil
}

func parseIqiyiProto(raw []byte) ([]domain.DanmakuData, error) {
	fields, err := parseIqiyiProtoFields(raw)
	if err != nil {
		return nil, err
	}
	contents := make([]string, 0)
	times := make([]string, 0)
	for _, field := range fields {
		if field.number != 6 || len(field.bytes) == 0 {
			continue
		}
		block, err := parseIqiyiProtoFields(field.bytes)
		if err != nil {
			continue
		}
		blockTime := iqiyiProtoString(block, 1)
		for _, itemField := range block {
			if itemField.number != 2 || len(itemField.bytes) == 0 {
				continue
			}
			item, err := parseIqiyiProtoFields(itemField.bytes)
			if err != nil {
				continue
			}
			content := iqiyiProtoString(item, 2)
			if content == "" {
				continue
			}
			showTime := iqiyiProtoString(item, 6)
			if showTime == "" {
				showTime = blockTime
			}
			contents = append(contents, content)
			times = append(times, showTime)
		}
	}
	return iqiyiDanmaku(contents, times, true), nil
}

func iqiyiDanmaku(contents, times []string, decodeHTML bool) []domain.DanmakuData {
	result := make([]domain.DanmakuData, 0, len(contents))
	for index, content := range contents {
		if index >= len(times) || content == "" {
			continue
		}
		showTime, err := strconv.ParseFloat(strings.TrimSpace(times[index]), 32)
		if err != nil || math.IsNaN(showTime) || math.IsInf(showTime, 0) {
			continue
		}
		if decodeHTML {
			content = html.UnescapeString(content)
		}
		data := domain.NewDanmakuData()
		data.Time = float32(showTime)
		data.Mode = 1
		data.Color = 16777215
		data.Text = stringPointer(content)
		result = append(result, data)
	}
	return result
}

func parseIqiyiProtoFields(raw []byte) ([]iqiyiProtoField, error) {
	fields := make([]iqiyiProtoField, 0)
	for offset := 0; offset < len(raw); {
		key, next, err := readIqiyiVarint(raw, offset)
		if err != nil {
			return nil, err
		}
		offset = next
		number, wireType := int(key>>3), int(key&7)
		if number == 0 {
			break
		}
		field := iqiyiProtoField{number: number}
		switch wireType {
		case 0:
			value, next, err := readIqiyiVarint(raw, offset)
			if err != nil {
				return nil, err
			}
			field.value = strconv.FormatUint(value, 10)
			offset = next
		case 1:
			if len(raw)-offset < 8 {
				return nil, errors.New("iqiyi protobuf fixed64 field is truncated")
			}
			offset += 8
		case 2:
			length, next, err := readIqiyiVarint(raw, offset)
			if err != nil {
				return nil, err
			}
			offset = next
			if length > uint64(len(raw)-offset) {
				return nil, errors.New("iqiyi protobuf bytes field is truncated")
			}
			end := offset + int(length)
			field.bytes = raw[offset:end]
			field.value = string(field.bytes)
			offset = end
		case 5:
			if len(raw)-offset < 4 {
				return nil, errors.New("iqiyi protobuf fixed32 field is truncated")
			}
			offset += 4
		default:
			return fields, nil
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func readIqiyiVarint(raw []byte, offset int) (uint64, int, error) {
	var value uint64
	for index := 0; index < 10 && offset+index < len(raw); index++ {
		current := raw[offset+index]
		if index == 9 && current > 1 {
			return 0, offset, errors.New("iqiyi protobuf varint overflows uint64")
		}
		value |= uint64(current&0x7f) << (7 * index)
		if current&0x80 == 0 {
			return value, offset + index + 1, nil
		}
	}
	return 0, offset, errors.New("iqiyi protobuf varint is truncated")
}

func iqiyiProtoString(fields []iqiyiProtoField, number int) string {
	for _, field := range fields {
		if field.number == number {
			return field.value
		}
	}
	return ""
}

func jsonScalarString(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	var value string
	if raw[0] == '"' && json.Unmarshal(raw, &value) == nil {
		return strings.TrimSpace(value)
	}
	for _, current := range raw {
		if current < '0' || current > '9' {
			return ""
		}
	}
	return string(raw)
}

func jsonScalarFloat(raw json.RawMessage) float64 {
	value := jsonScalarString(raw)
	if value == "" {
		value = strings.TrimSpace(string(raw))
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0
	}
	return parsed
}

func iqiyiDPlayerRows(data []domain.DanmakuData) [][]any {
	rows := make([][]any, 0, len(data))
	for _, item := range data {
		text := ""
		if item.Text != nil {
			text = *item.Text
		}
		rows = append(rows, []any{item.Time, 0, item.Color, "", text})
	}
	return rows
}
