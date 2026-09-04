package store

import "testing"

func TestParseReferer(t *testing.T) {
	referer, err := ParseReferer("https://example.com/watch/index.html?id=7#player")
	if err != nil {
		t.Fatal(err)
	}
	if referer.Protocol != "https" || referer.Host != "example.com" || referer.Port != 443 || referer.Path != "/watch/index.html" || referer.Query != "?id=7" || referer.Fragment != "#player" {
		t.Fatalf("unexpected referer: %#v", referer)
	}
}

func TestParseRefererRootPath(t *testing.T) {
	referer, err := ParseReferer("https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if referer.Path != "/" {
		t.Fatalf("unexpected root path: %q", referer.Path)
	}
}

func TestSaltedMD5MatchesLegacyAlgorithm(t *testing.T) {
	if got := saltedMD5("670b14728ad9902aecba32e22fa4f6bd", "Ab12Cd"); got != "d5b4c34886d55adb71f84f690c855044" {
		t.Fatalf("legacy hash changed: %s", got)
	}
}

func TestRandomUUID(t *testing.T) {
	value, err := randomUUID()
	if err != nil {
		t.Fatal(err)
	}
	if !validUUID(value) {
		t.Fatalf("invalid UUID: %s", value)
	}
}
