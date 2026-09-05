package store

import (
	"errors"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"net"
	"testing"
)

func TestNormalizeNativeRule(t *testing.T) {
	for _, tt := range []struct {
		kind, value, replacement, want string
		scan, invalid                  bool
	}{
		{kind: "keywords", value: "  SPAM 中文  ", want: "spam 中文"},
		{kind: "authors", value: " Alice ", replacement: "Bob", want: " Alice ", scan: true},
		{kind: "ips", value: "192.0.2.129/24", want: "192.0.2.0/24"},
		{kind: "ips", value: "192.0.2.1", want: "192.0.2.1/32"},
		{kind: "ips", value: "2001:db8::ab/64", want: "2001:db8::/64"},
		{kind: "ips", value: "::1", want: "::1/128"},
		{kind: "ips", value: "::ffff:192.0.2.129/120", want: "192.0.2.0/24"},
		{kind: "ips", value: "0.0.0.0/0", want: "0.0.0.0/0"},
		{kind: "ips", value: "192.0.2.1/33", invalid: true},
		{kind: "ips", value: "192.0.2.1-192.0.2.10", invalid: true},
		{kind: "ips", value: "fe80::1%eth0", invalid: true},
		{kind: "ips", value: "::ffff:192.0.2.1/64", invalid: true},
		{kind: "keywords", value: "", invalid: true},
		{kind: "keywords", value: "spam", scan: true, invalid: true},
		{kind: "authors", value: "a", replacement: "a", invalid: true},
		{kind: "authors", value: "a", replacement: " ", invalid: true},
		{kind: "unknown", value: "a", invalid: true},
	} {
		t.Run(tt.kind+tt.value, func(t *testing.T) {
			got, err := normalizeNativeRule(tt.kind, NativeRuleInput{Value: tt.value, Replacement: tt.replacement, ScanExisting: tt.scan})
			if tt.invalid {
				if !errors.Is(err, ErrInvalidNativeRule) {
					t.Fatalf("err=%v", err)
				}
				return
			}
			if err != nil || got.Value != tt.want {
				t.Fatalf("got=%#v err=%v want=%q", got, err, tt.want)
			}
		})
	}
}

func TestApplyNativeRules(t *testing.T) {
	text := "a SpAm message 中文"
	original := domain.DanmakuData{Author: "a", AuthorID: 7, Text: &text, Time: 2}
	rules := []NativeRule{{Kind: "authors", Value: "a", Replacement: "b"}, {Kind: "authors", Value: "b", Replacement: "c"}, {Kind: "keywords", Value: "spam"}}
	got, deleted, err := applyNativeRules(original, net.ParseIP("192.0.2.1"), rules)
	if err != nil || !deleted || got.Author != "b" || got.AuthorID != 7 || got.Time != 2 || *got.Text != text || original.Author != "a" {
		t.Fatalf("got=%#v deleted=%v err=%v", got, deleted, err)
	}
	rules = append(rules, NativeRule{Kind: "ips", Value: "192.0.2.0/24"})
	if _, _, err := applyNativeRules(original, net.ParseIP("192.0.2.1"), rules); !errors.Is(err, ErrSubmissionDenied) {
		t.Fatalf("blacklist must override keyword success: %v", err)
	}
	for _, tt := range []struct {
		network, ip string
		blocked     bool
	}{
		{"192.0.2.0/24", "192.0.2.0", true}, {"192.0.2.0/24", "192.0.2.255", true}, {"192.0.2.0/24", "192.0.3.0", false},
		{"192.0.2.1/32", "::ffff:192.0.2.1", true}, {"2001:db8::/32", "2001:db8::ffff", true}, {"2001:db8::/32", "2001:db9::1", false},
		{"0.0.0.0/0", "2001:db8::1", false},
	} {
		t.Run(tt.network+tt.ip, func(t *testing.T) {
			_, _, err := applyNativeRules(original, net.ParseIP(tt.ip), []NativeRule{{Kind: "ips", Value: tt.network}})
			if errors.Is(err, ErrSubmissionDenied) != tt.blocked {
				t.Fatalf("err=%v blocked=%v", err, tt.blocked)
			}
		})
	}
	got, deleted, err = applyNativeRules(domain.DanmakuData{AuthorID: 123}, nil, []NativeRule{{Kind: "authors", Value: "123", Replacement: "numeric"}})
	if err != nil || deleted || got.Author != "numeric" {
		t.Fatalf("numeric display author mapping=%#v %v", got, err)
	}
	got, deleted, err = applyNativeRules(domain.DanmakuData{Author: "A"}, nil, rules[:3])
	if err != nil || deleted || got.Author != "A" {
		t.Fatalf("author match must be exact: %#v %v", got, err)
	}
}
