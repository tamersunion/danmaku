package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMarshalDBDataPreservesExistingJSONKeys(t *testing.T) {
	text := "hello"
	raw, err := MarshalDBData(DanmakuData{Time: 1.5, Mode: 4, Size: 25, Color: 16777215, Timestamp: 123, Pool: 0, Author: "alice", AuthorID: 7, Text: &text})
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(raw)
	for _, key := range []string{`"Time"`, `"Mode"`, `"Size"`, `"Color"`, `"TimeStamp"`, `"Pool"`, `"Author"`, `"AuthorId"`, `"Text"`} {
		if !strings.Contains(serialized, key) {
			t.Fatalf("database JSON %s does not contain %s", serialized, key)
		}
	}
	if strings.Contains(serialized, `"time"`) || strings.Contains(serialized, `"authorId"`) {
		t.Fatalf("database JSON keys changed: %s", serialized)
	}

	var decoded DanmakuData
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Mode != 4 || decoded.AuthorID != 7 || decoded.Text == nil || *decoded.Text != text {
		t.Fatalf("unexpected round trip: %#v", decoded)
	}
}

func TestExternalJSONUsesCamelCase(t *testing.T) {
	text := "hello"
	raw, err := json.Marshal(DanmakuData{Timestamp: 123, AuthorID: 7, Text: &text})
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(raw)
	if !strings.Contains(serialized, `"timeStamp":123`) || !strings.Contains(serialized, `"authorId":7`) {
		t.Fatalf("unexpected API JSON: %s", serialized)
	}
}

func TestDanmakuVideoPreservesExistingResponseKeys(t *testing.T) {
	raw, err := json.Marshal(DanmakuVideo{ID: 1, Vid: "video", UpdateTime: time.Unix(0, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(raw)
	for _, key := range []string{`"id"`, `"vid"`, `"referer"`, `"createTime"`, `"upDateTime"`} {
		if !strings.Contains(serialized, key) {
			t.Fatalf("existing danmaku video response %s does not contain %s", serialized, key)
		}
	}
	if strings.Contains(serialized, `"updateTime"`) {
		t.Fatalf("existing danmaku video response key changed: %s", serialized)
	}
}
