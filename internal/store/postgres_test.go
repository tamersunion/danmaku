package store

import (
	"net"
	"strings"
	"testing"
	"time"

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
		`CREATE INDEX IF NOT EXISTS "IX_Danmaku_DuplicateGuard"`,
		`CREATE TABLE IF NOT EXISTS "BilibiliDanmakuPool"`,
		`CREATE TABLE IF NOT EXISTS "BilibiliDanmaku"`,
		`CREATE TABLE IF NOT EXISTS "BilibiliDanmakuKeyword"`,
		`CREATE TABLE IF NOT EXISTS "BilibiliDanmakuBinding"`,
		`CREATE TABLE IF NOT EXISTS "IqiyiDanmakuPool"`,
		`CREATE TABLE IF NOT EXISTS "IqiyiDanmaku"`,
		`CREATE TABLE IF NOT EXISTS "IqiyiDanmakuKeyword"`,
		`CREATE TABLE IF NOT EXISTS "IqiyiDanmakuBinding"`,
		`CREATE TABLE IF NOT EXISTS "DandanplayDanmakuPool"`,
		`CREATE TABLE IF NOT EXISTS "DandanplayDanmaku"`,
		`CREATE TABLE IF NOT EXISTS "DandanplayDanmakuKeyword"`,
		`CREATE TABLE IF NOT EXISTS "DandanplayDanmakuBinding"`,
		`"EpisodeId" character varying(128) NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS "UX_DandanplayDanmakuPool_EpisodeId"`,
		`CREATE UNIQUE INDEX IF NOT EXISTS "UX_DandanplayDanmaku_Pool_TimeMillis_ContentHash"`,
		`CONSTRAINT "FK_DandanplayDanmakuBinding_Video_Vid"`,
		`CREATE TABLE IF NOT EXISTS "ExternalDanmakuPool"`,
		`CREATE TABLE IF NOT EXISTS "ExternalDanmaku"`,
		`CREATE TABLE IF NOT EXISTS "ExternalDanmakuBinding"`,
		`CREATE UNIQUE INDEX IF NOT EXISTS "UX_BilibiliDanmaku_Pool_Timestamp_ContentHash"`,
		`CREATE UNIQUE INDEX IF NOT EXISTS "UX_BilibiliDanmakuPool_CID"`,
		`CREATE INDEX IF NOT EXISTS "IX_BilibiliDanmakuPool_BVID_Page"`,
		`ALTER TABLE "BilibiliDanmakuPool" ALTER COLUMN "BVID" DROP NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS "UX_BilibiliDanmakuKeyword_Global"`,
		`CREATE UNIQUE INDEX IF NOT EXISTS "UX_BilibiliDanmakuBinding_Vid_Pool"`,
		`"LastAttemptTime" timestamp(3) without time zone NULL`,
		`CONSTRAINT "FK_BilibiliDanmakuBinding_Pool_PoolId"`,
		`CONSTRAINT "FK_IqiyiDanmakuBinding_Pool_PoolId"`,
		`CONSTRAINT "FK_IqiyiDanmakuBinding_Video_Vid"`,
		`CREATE UNIQUE INDEX IF NOT EXISTS "UX_ExternalDanmaku_Pool_TimeMillis_ContentHash"`,
		`CREATE UNIQUE INDEX IF NOT EXISTS "UX_ExternalDanmakuBinding_Vid_Pool"`,
		`CONSTRAINT "FK_ExternalDanmakuBinding_Video_Vid"`,
		`ALTER TABLE "Video" RENAME COLUMN "UpDateTime" TO "UpdateTime"`,
		`ALTER TABLE "Video" ADD COLUMN IF NOT EXISTS "Name"`,
		`ALTER TABLE "Video" ADD COLUMN IF NOT EXISTS "IsDelete"`,
		`CREATE UNIQUE INDEX IF NOT EXISTS "UX_Video_Vid"`,
		`UPDATE "Danmaku" d SET "VideoId"=v."Id"`,
		`CONSTRAINT "FK_BilibiliDanmakuBinding_Video_Vid"`,
	} {
		if !strings.Contains(migration, expected) {
			t.Fatalf("schema migration does not contain %q", expected)
		}
	}
	if !strings.Contains(migration, `both legacy table "Danmu" and target table "Danmaku" exist`) {
		t.Fatal("schema migration must reject ambiguous old/new table state")
	}
}

func TestVideoMigrationOrdersDataRepairBeforeConstraints(t *testing.T) {
	migration := strings.Join(schemaStatements(), "\n")
	ordered := []string{
		`ALTER TABLE "Video" RENAME COLUMN "UpDateTime" TO "UpdateTime"`,
		`INSERT INTO "Video" ("Vid","Referer","Name","IsDelete","CreateTime","UpdateTime")`,
		`FROM "ExternalDanmakuBinding" b`,
		`UPDATE "Danmaku" d SET "VideoId"=c."Id"`,
		`DELETE FROM "Video" duplicate`,
		`UPDATE "Danmaku" d SET "VideoId"=v."Id"`,
		`CREATE UNIQUE INDEX IF NOT EXISTS "UX_Video_Vid"`,
		`CONSTRAINT "FK_BilibiliDanmakuBinding_Video_Vid"`,
		`CONSTRAINT "FK_ExternalDanmakuBinding_Video_Vid"`,
	}
	previous := -1
	for _, fragment := range ordered {
		index := strings.Index(migration, fragment)
		if index < 0 {
			t.Fatalf("video migration does not contain %q", fragment)
		}
		if index <= previous {
			t.Fatalf("video migration fragment %q is out of order", fragment)
		}
		previous = index
	}
}

func TestBilibiliKeywordFilterKeepsGlobalAndPoolScopes(t *testing.T) {
	for _, expected := range []string{
		`k."PoolId" IS NULL`,
		`k."PoolId"=d."PoolId"`,
		`strpos(lower(d."Content"), lower(k."Keyword"))>0`,
	} {
		if !strings.Contains(bilibiliKeywordBlockedSQL, expected) {
			t.Fatalf("Bilibili keyword filter does not contain %q", expected)
		}
	}
}

func TestDuplicateDanmakuGuardContract(t *testing.T) {
	if duplicateDanmakuWindow != 30*time.Second {
		t.Fatalf("duplicate window = %s", duplicateDanmakuWindow)
	}
	for _, expected := range []string{
		`"Vid"=$1`,
		`"Ip" IS NOT DISTINCT FROM $2::inet`,
		`COALESCE("Data"->>'Text','')=$3`,
		`"CreateTime">=$4`,
	} {
		if !strings.Contains(duplicateDanmakuQuery, expected) {
			t.Fatalf("duplicate query does not contain %q", expected)
		}
	}

	base := duplicateDanmakuAdvisoryKey("video", net.ParseIP("2001:db8::1"), "hello")
	if base != duplicateDanmakuAdvisoryKey("video", net.ParseIP("2001:0db8:0:0:0:0:0:1"), "hello") {
		t.Fatal("equivalent IP addresses must use the same advisory lock")
	}
	for name, key := range map[string]int64{
		"IP":      duplicateDanmakuAdvisoryKey("video", net.ParseIP("2001:db8::2"), "hello"),
		"VID":     duplicateDanmakuAdvisoryKey("other", net.ParseIP("2001:db8::1"), "hello"),
		"content": duplicateDanmakuAdvisoryKey("video", net.ParseIP("2001:db8::1"), "other"),
	} {
		if base == key {
			t.Fatalf("different %s must use a different advisory lock", name)
		}
	}
}
