package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
)

const externalKeywordBlockedSQL = `EXISTS (SELECT 1 FROM "ExternalDanmakuKeyword" k WHERE (k."PoolId" IS NULL OR k."PoolId"=d."PoolId") AND strpos(lower(d."Content"),lower(k."Keyword"))>0)`

func (p *Postgres) ExternalKeywords(ctx context.Context) ([]domain.ExternalKeyword, error) {
	rows, err := p.pool.Query(ctx, `SELECT k."Id",k."PoolId"::text,COALESCE(p."Name",''),k."Keyword",k."CreateTime" FROM "ExternalDanmakuKeyword" k LEFT JOIN "ExternalDanmakuPool" p ON p."Id"=k."PoolId" ORDER BY k."Id" DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []domain.ExternalKeyword{}
	for rows.Next() {
		var item domain.ExternalKeyword
		if err := rows.Scan(&item.ID, &item.PoolID, &item.PoolName, &item.Keyword, &item.CreateTime); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (p *Postgres) CreateExternalKeyword(ctx context.Context, poolID *string, keyword string) (*domain.ExternalKeyword, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || len([]rune(keyword)) > 200 {
		return nil, fmt.Errorf("invalid keyword")
	}
	if poolID != nil && !validUUID(*poolID) {
		return nil, fmt.Errorf("invalid pool ID")
	}
	hash := sha256.Sum256([]byte(strings.ToLower(keyword)))
	conflict := `("KeywordHash") WHERE "PoolId" IS NULL`
	if poolID != nil {
		conflict = `("PoolId","KeywordHash") WHERE "PoolId" IS NOT NULL`
	}
	var item domain.ExternalKeyword
	err := p.pool.QueryRow(ctx, `INSERT INTO "ExternalDanmakuKeyword" ("PoolId","Keyword","KeywordHash","CreateTime") VALUES ($1,$2,$3,$4) ON CONFLICT `+conflict+` DO UPDATE SET "Keyword"=EXCLUDED."Keyword" RETURNING "Id","PoolId"::text,"Keyword","CreateTime"`, poolID, keyword, hex.EncodeToString(hash[:]), time.Now().UTC()).Scan(&item.ID, &item.PoolID, &item.Keyword, &item.CreateTime)
	if err != nil {
		return nil, err
	}
	p.invalidateDanmakuCache(ctx, "external")
	return &item, nil
}

func (p *Postgres) DeleteExternalKeyword(ctx context.Context, id int) (bool, error) {
	tag, err := p.pool.Exec(ctx, `DELETE FROM "ExternalDanmakuKeyword" WHERE "Id"=$1`, id)
	if err == nil && tag.RowsAffected() > 0 {
		p.invalidateDanmakuCache(ctx, "external")
	}
	return tag.RowsAffected() > 0, err
}
