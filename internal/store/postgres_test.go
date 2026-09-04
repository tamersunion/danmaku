package store

import (
	"strings"
	"testing"

	"git.hanada.info/tamersunion/danmaku/internal/config"
)

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

func TestConfiguredAdminMatchesFrontendPasswordHash(t *testing.T) {
	admin := config.AdminSettings{User: "tamers", Password: "tamersunion2022"}
	password := md5String(admin.Password)
	if !configuredAdminMatches(admin, "tamers", password) {
		t.Fatal("configured administrator credentials should match without a database user")
	}
	if configuredAdminMatches(admin, "other", password) {
		t.Fatal("different user must not match configured administrator")
	}
	if configuredAdminMatches(config.AdminSettings{}, "", md5String("")) {
		t.Fatal("empty administrator configuration must not enable login")
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

func TestSchemaMigrationRenamesDanmakuDatabaseObjects(t *testing.T) {
	migration := strings.Join(schemaStatements(), "\n")
	for _, expected := range []string{
		`ALTER TABLE "Danmu" RENAME TO "Danmaku"`,
		`CREATE TABLE IF NOT EXISTS "Danmaku"`,
		`RENAME CONSTRAINT "FK_Danmu_Video_VideoId" TO "FK_Danmaku_Video_VideoId"`,
		`ALTER INDEX "IX_Danmu_Vid_IsDelete" RENAME TO "IX_Danmaku_Vid_IsDelete"`,
		`"CASSubject"`,
		`"UX_User_CASSubject"`,
		`"Enabled" boolean NOT NULL DEFAULT TRUE`,
	} {
		if !strings.Contains(migration, expected) {
			t.Fatalf("schema migration does not contain %q", expected)
		}
	}
	if !strings.Contains(migration, `both legacy table "Danmu" and target table "Danmaku" exist`) {
		t.Fatal("schema migration must reject ambiguous old/new table state")
	}
}
