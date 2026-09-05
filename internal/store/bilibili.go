package store

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"github.com/jackc/pgx/v5"
)

const bilibiliKeywordBlockedSQL = `EXISTS (
	SELECT 1
	FROM "BilibiliDanmakuKeyword" k
	WHERE (k."PoolId" IS NULL OR k."PoolId"=d."PoolId")
		AND strpos(lower(d."Content"), lower(k."Keyword"))>0
)`

const bilibiliPoolSelect = `SELECT "Id",COALESCE("BVID",''),"Page","CID","LastAttemptTime","LastSyncTime","CreateTime","UpdateTime" FROM "BilibiliDanmakuPool"`

func (p *Postgres) BilibiliPool(ctx context.Context, id int) (*domain.BilibiliPool, error) {
	return scanBilibiliPool(p.pool.QueryRow(ctx, bilibiliPoolSelect+` WHERE "Id"=$1`, id))
}

func (p *Postgres) BilibiliPoolByKey(ctx context.Context, bvid string, page int) (*domain.BilibiliPool, error) {
	if bvid == "" {
		return nil, nil
	}
	pool, err := scanBilibiliPool(p.pool.QueryRow(ctx, bilibiliPoolSelect+` WHERE "BVID"=$1 AND "Page"=$2 ORDER BY "Id" DESC LIMIT 1`, bvid, page))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return pool, err
}

func (p *Postgres) EnsureBilibiliPool(ctx context.Context, bvid string, page int, cid int64) (*domain.BilibiliPool, error) {
	if page < 1 || cid < 1 || len(bvid) > 32 {
		return nil, fmt.Errorf("invalid Bilibili pool identity")
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, bilibiliPoolAdvisoryKey(cid)); err != nil {
		return nil, err
	}

	pool, err := scanBilibiliPool(tx.QueryRow(ctx, bilibiliPoolSelect+` WHERE "CID"=$1`, cid))
	now := time.Now().UTC().Truncate(time.Millisecond)
	if err == nil {
		newBVID, newPage := pool.BVID, pool.Page
		if pool.BVID == "" && bvid != "" {
			newBVID, newPage = bvid, page
		}
		if pool.CID != cid || pool.BVID != newBVID || pool.Page != newPage {
			if _, err := tx.Exec(ctx, `UPDATE "BilibiliDanmakuPool" SET "BVID"=NULLIF($2,''),"Page"=$3,"CID"=$4,"UpdateTime"=$5 WHERE "Id"=$1`, pool.ID, newBVID, newPage, cid, now); err != nil {
				return nil, err
			}
			pool.BVID, pool.Page, pool.CID, pool.UpdateTime = newBVID, newPage, cid, now
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		pool = &domain.BilibiliPool{BVID: bvid, Page: page, CID: cid, CreateTime: now, UpdateTime: now}
		if err := tx.QueryRow(ctx, `INSERT INTO "BilibiliDanmakuPool" ("BVID","Page","CID","CreateTime","UpdateTime") VALUES (NULLIF($1,''),$2,$3,$4,$4) RETURNING "Id"`, bvid, page, cid, now).Scan(&pool.ID); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return pool, nil
}

func (p *Postgres) ClaimBilibiliPoolSync(ctx context.Context, poolID int, interval time.Duration, force bool) (bool, error) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	cutoff := now.Add(-interval)
	tag, err := p.pool.Exec(ctx, `UPDATE "BilibiliDanmakuPool" SET "LastAttemptTime"=$2,"UpdateTime"=$2 WHERE "Id"=$1 AND ($3 OR "LastAttemptTime" IS NULL OR "LastAttemptTime"<=$4)`, poolID, now, force, cutoff)
	return tag.RowsAffected() > 0, err
}

func (p *Postgres) MergeBilibiliDanmaku(ctx context.Context, poolID int, data []domain.DanmakuData) (int, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := time.Now().UTC().Truncate(time.Millisecond)
	inserted := 0
	if len(data) > 0 {
		batch := &pgx.Batch{}
		for _, item := range data {
			content := ""
			if item.Text != nil {
				content = *item.Text
			}
			raw, err := domain.MarshalDBData(item)
			if err != nil {
				return 0, err
			}
			hash := sha256.Sum256([]byte(content))
			batch.Queue(`INSERT INTO "BilibiliDanmaku" ("PoolId","Timestamp","Content","ContentHash","Data","IsBlocked","CreateTime","UpdateTime") VALUES ($1,$2,$3,$4,$5,FALSE,$6,$6) ON CONFLICT ("PoolId","Timestamp","ContentHash") DO NOTHING`, poolID, item.Timestamp, content, hex.EncodeToString(hash[:]), raw, now)
		}
		results := tx.SendBatch(ctx, batch)
		for range data {
			tag, execErr := results.Exec()
			if execErr != nil {
				_ = results.Close()
				return 0, execErr
			}
			inserted += int(tag.RowsAffected())
		}
		if err := results.Close(); err != nil {
			return 0, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE "BilibiliDanmakuPool" SET "LastSyncTime"=$2,"UpdateTime"=$2 WHERE "Id"=$1`, poolID, now); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return inserted, nil
}

func (p *Postgres) BilibiliPoolData(ctx context.Context, poolID int) ([]domain.DanmakuData, error) {
	query := `SELECT d."Data" FROM "BilibiliDanmaku" d WHERE d."PoolId"=$1 AND NOT d."IsBlocked" AND NOT ` + bilibiliKeywordBlockedSQL + ` ORDER BY d."Id"`
	rows, err := p.pool.Query(ctx, query, poolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.DanmakuData, 0)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item domain.DanmakuData
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (p *Postgres) BilibiliPools(ctx context.Context, filter BilibiliPoolFilter) (domain.Page[domain.BilibiliPool], error) {
	filter.Page, filter.Size = normalizePage(filter.Page, filter.Size)
	where := ""
	args := []any{}
	if strings.TrimSpace(filter.Query) != "" {
		args = append(args, strings.TrimSpace(filter.Query))
		where = ` WHERE p."BVID" ILIKE '%' || $1 || '%' OR p."CID"::text=$1`
	}
	var total int
	if err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM "BilibiliDanmakuPool" p`+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.BilibiliPool]{}, err
	}
	args = append(args, filter.Size, filter.Size*(filter.Page-1))
	query := `SELECT p."Id",COALESCE(p."BVID",''),p."Page",p."CID",p."LastAttemptTime",p."LastSyncTime",p."CreateTime",p."UpdateTime",
		(SELECT COUNT(*)::integer FROM "BilibiliDanmaku" d WHERE d."PoolId"=p."Id"),
		(SELECT COUNT(*)::integer FROM "BilibiliDanmaku" d WHERE d."PoolId"=p."Id" AND (d."IsBlocked" OR ` + bilibiliKeywordBlockedSQL + `)),
		(SELECT COUNT(*)::integer FROM "BilibiliDanmakuBinding" b WHERE b."PoolId"=p."Id")
		FROM "BilibiliDanmakuPool" p` + where + `
		ORDER BY p."BVID",p."Page"
		LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return domain.Page[domain.BilibiliPool]{}, err
	}
	defer rows.Close()
	list := make([]domain.BilibiliPool, 0)
	for rows.Next() {
		var item domain.BilibiliPool
		if err := rows.Scan(&item.ID, &item.BVID, &item.Page, &item.CID, &item.LastAttemptTime, &item.LastSyncTime, &item.CreateTime, &item.UpdateTime, &item.DanmakuCount, &item.BlockedCount, &item.BindingCount); err != nil {
			return domain.Page[domain.BilibiliPool]{}, err
		}
		list = append(list, item)
	}
	return domain.Page[domain.BilibiliPool]{Total: total, List: list}, rows.Err()
}

func (p *Postgres) BilibiliDanmaku(ctx context.Context, filter BilibiliDanmakuFilter) (domain.Page[domain.BilibiliDanmaku], error) {
	filter.Page, filter.Size = normalizePage(filter.Page, filter.Size)
	blocked := `(d."IsBlocked" OR ` + bilibiliKeywordBlockedSQL + `)`
	clauses := []string{`d."PoolId"=$1`}
	args := []any{filter.PoolID}
	if strings.TrimSpace(filter.Query) != "" {
		args = append(args, strings.TrimSpace(filter.Query))
		clauses = append(clauses, fmt.Sprintf(`d."Content" ILIKE '%%' || $%d || '%%'`, len(args)))
	}
	if filter.Blocked != nil {
		args = append(args, *filter.Blocked)
		clauses = append(clauses, fmt.Sprintf(`%s=$%d`, blocked, len(args)))
	}
	where := " WHERE " + strings.Join(clauses, " AND ")
	var total int
	if err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM "BilibiliDanmaku" d`+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.BilibiliDanmaku]{}, err
	}
	args = append(args, filter.Size, filter.Size*(filter.Page-1))
	query := `SELECT d."Id",d."PoolId",d."Data",` + blocked + `,d."IsBlocked",d."CreateTime",d."UpdateTime" FROM "BilibiliDanmaku" d` + where + ` ORDER BY d."Timestamp" DESC,d."Id" DESC LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return domain.Page[domain.BilibiliDanmaku]{}, err
	}
	defer rows.Close()
	list := make([]domain.BilibiliDanmaku, 0)
	for rows.Next() {
		var item domain.BilibiliDanmaku
		var raw []byte
		if err := rows.Scan(&item.ID, &item.PoolID, &raw, &item.IsBlocked, &item.ManuallyBlocked, &item.CreateTime, &item.UpdateTime); err != nil {
			return domain.Page[domain.BilibiliDanmaku]{}, err
		}
		if err := json.Unmarshal(raw, &item.Data); err != nil {
			return domain.Page[domain.BilibiliDanmaku]{}, err
		}
		list = append(list, item)
	}
	return domain.Page[domain.BilibiliDanmaku]{Total: total, List: list}, rows.Err()
}

func (p *Postgres) SetBilibiliDanmakuBlocked(ctx context.Context, id int64, blocked bool) (bool, error) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	tag, err := p.pool.Exec(ctx, `UPDATE "BilibiliDanmaku" SET "IsBlocked"=$2,"UpdateTime"=$3 WHERE "Id"=$1`, id, blocked, now)
	return tag.RowsAffected() > 0, err
}

func (p *Postgres) BilibiliKeywords(ctx context.Context) ([]domain.BilibiliKeyword, error) {
	rows, err := p.pool.Query(ctx, `SELECT k."Id",k."PoolId",COALESCE(p."BVID",''),COALESCE(p."Page",0),COALESCE(p."CID",0),k."Keyword",k."CreateTime" FROM "BilibiliDanmakuKeyword" k LEFT JOIN "BilibiliDanmakuPool" p ON p."Id"=k."PoolId" ORDER BY k."PoolId" NULLS FIRST,k."Keyword"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.BilibiliKeyword, 0)
	for rows.Next() {
		var item domain.BilibiliKeyword
		if err := rows.Scan(&item.ID, &item.PoolID, &item.PoolBVID, &item.PoolPage, &item.PoolCID, &item.Keyword, &item.CreateTime); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (p *Postgres) CreateBilibiliKeyword(ctx context.Context, poolID *int, keyword string) (*domain.BilibiliKeyword, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, fmt.Errorf("keyword cannot be empty")
	}
	if poolID != nil && *poolID < 1 {
		return nil, fmt.Errorf("invalid Bilibili pool")
	}
	hash := sha256.Sum256([]byte(strings.ToLower(keyword)))
	hashText := hex.EncodeToString(hash[:])
	now := time.Now().UTC().Truncate(time.Millisecond)
	var poolValue any
	if poolID != nil {
		poolValue = *poolID
	}
	var item domain.BilibiliKeyword
	err := p.pool.QueryRow(ctx, `INSERT INTO "BilibiliDanmakuKeyword" ("PoolId","Keyword","KeywordHash","CreateTime") VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING RETURNING "Id","PoolId","Keyword","CreateTime"`, poolValue, keyword, hashText, now).Scan(&item.ID, &item.PoolID, &item.Keyword, &item.CreateTime)
	if errors.Is(err, pgx.ErrNoRows) {
		err = p.pool.QueryRow(ctx, `SELECT "Id","PoolId","Keyword","CreateTime" FROM "BilibiliDanmakuKeyword" WHERE "PoolId" IS NOT DISTINCT FROM $1::integer AND "KeywordHash"=$2`, poolValue, hashText).Scan(&item.ID, &item.PoolID, &item.Keyword, &item.CreateTime)
	}
	if err != nil {
		return nil, err
	}
	if item.PoolID != nil {
		pool, err := p.BilibiliPool(ctx, *item.PoolID)
		if err != nil {
			return nil, err
		}
		item.PoolBVID, item.PoolPage, item.PoolCID = pool.BVID, pool.Page, pool.CID
	}
	return &item, nil
}

func (p *Postgres) DeleteBilibiliKeyword(ctx context.Context, id int) (bool, error) {
	tag, err := p.pool.Exec(ctx, `DELETE FROM "BilibiliDanmakuKeyword" WHERE "Id"=$1`, id)
	return tag.RowsAffected() > 0, err
}

func (p *Postgres) BilibiliBindings(ctx context.Context, filter BilibiliBindingFilter) (domain.Page[domain.BilibiliBinding], error) {
	filter.Page, filter.Size = normalizePage(filter.Page, filter.Size)
	where := ""
	args := []any{}
	if strings.TrimSpace(filter.Query) != "" {
		args = append(args, strings.TrimSpace(filter.Query))
		where = ` WHERE b."Vid" ILIKE '%' || $1 || '%' OR p."BVID" ILIKE '%' || $1 || '%'`
	}
	var total int
	if err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM "BilibiliDanmakuBinding" b JOIN "BilibiliDanmakuPool" p ON p."Id"=b."PoolId"`+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.BilibiliBinding]{}, err
	}
	args = append(args, filter.Size, filter.Size*(filter.Page-1))
	query := bilibiliBindingSelect + where + ` ORDER BY b."Vid",p."BVID",p."Page" LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return domain.Page[domain.BilibiliBinding]{}, err
	}
	defer rows.Close()
	list, err := scanBilibiliBindings(rows)
	return domain.Page[domain.BilibiliBinding]{Total: total, List: list}, err
}

func (p *Postgres) BilibiliBindingsByVID(ctx context.Context, vid string) ([]domain.BilibiliBinding, error) {
	rows, err := p.pool.Query(ctx, bilibiliBindingSelect+` WHERE b."Vid"=$1 ORDER BY b."Id"`, vid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBilibiliBindings(rows)
}

func (p *Postgres) UpsertBilibiliBinding(ctx context.Context, vid string, poolID int, offset float64) (*domain.BilibiliBinding, error) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	var id int
	err := p.pool.QueryRow(ctx, `INSERT INTO "BilibiliDanmakuBinding" ("Vid","PoolId","Offset","CreateTime","UpdateTime") VALUES ($1,$2,$3,$4,$4) ON CONFLICT ("Vid","PoolId") DO UPDATE SET "Offset"=EXCLUDED."Offset","UpdateTime"=EXCLUDED."UpdateTime" RETURNING "Id"`, vid, poolID, offset, now).Scan(&id)
	if err != nil {
		return nil, err
	}
	return scanBilibiliBinding(p.pool.QueryRow(ctx, bilibiliBindingSelect+` WHERE b."Id"=$1`, id))
}

func (p *Postgres) DeleteBilibiliBinding(ctx context.Context, id int) (bool, error) {
	tag, err := p.pool.Exec(ctx, `DELETE FROM "BilibiliDanmakuBinding" WHERE "Id"=$1`, id)
	return tag.RowsAffected() > 0, err
}

const bilibiliBindingSelect = `SELECT b."Id",b."Vid",b."PoolId",COALESCE(p."BVID",''),p."Page",p."CID",b."Offset",b."CreateTime",b."UpdateTime" FROM "BilibiliDanmakuBinding" b JOIN "BilibiliDanmakuPool" p ON p."Id"=b."PoolId"`

func scanBilibiliPool(row pgx.Row) (*domain.BilibiliPool, error) {
	var item domain.BilibiliPool
	err := row.Scan(&item.ID, &item.BVID, &item.Page, &item.CID, &item.LastAttemptTime, &item.LastSyncTime, &item.CreateTime, &item.UpdateTime)
	return &item, err
}

func scanBilibiliBinding(row pgx.Row) (*domain.BilibiliBinding, error) {
	var item domain.BilibiliBinding
	err := row.Scan(&item.ID, &item.Vid, &item.PoolID, &item.BVID, &item.Page, &item.CID, &item.Offset, &item.CreateTime, &item.UpdateTime)
	return &item, err
}

func scanBilibiliBindings(rows pgx.Rows) ([]domain.BilibiliBinding, error) {
	result := make([]domain.BilibiliBinding, 0)
	for rows.Next() {
		item, err := scanBilibiliBinding(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	return result, rows.Err()
}

func bilibiliPoolAdvisoryKey(cid int64) int64 {
	hash := sha256.New()
	var size [8]byte
	for _, value := range []string{"danmaku:bilibili-pool:v1", fmt.Sprint(cid)} {
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write([]byte(value))
	}
	sum := hash.Sum(nil)
	return int64(binary.BigEndian.Uint64(sum[:8]))
}
