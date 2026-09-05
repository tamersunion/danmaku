package store

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"github.com/jackc/pgx/v5"
)

const iqiyiKeywordBlockedSQL = `EXISTS (
	SELECT 1
	FROM "IqiyiDanmakuKeyword" k
	WHERE (k."PoolId" IS NULL OR k."PoolId"=d."PoolId")
		AND strpos(lower(d."Content"), lower(k."Keyword"))>0
)`

const iqiyiPoolSelect = `SELECT "Id","VID","LastAttemptTime","LastSyncTime","CreateTime","UpdateTime" FROM "IqiyiDanmakuPool"`

func (p *Postgres) IqiyiPool(ctx context.Context, id int) (*domain.IqiyiPool, error) {
	pool, err := scanIqiyiPool(p.pool.QueryRow(ctx, iqiyiPoolSelect+` WHERE "Id"=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return pool, err
}

func (p *Postgres) EnsureIqiyiPool(ctx context.Context, vid string) (*domain.IqiyiPool, error) {
	vid = strings.TrimSpace(vid)
	if vid == "" || len(vid) > 128 {
		return nil, fmt.Errorf("invalid iQiyi pool identity")
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, iqiyiPoolAdvisoryKey(vid)); err != nil {
		return nil, err
	}
	pool, err := scanIqiyiPool(tx.QueryRow(ctx, iqiyiPoolSelect+` WHERE "VID"=$1`, vid))
	if errors.Is(err, pgx.ErrNoRows) {
		now := time.Now().UTC().Truncate(time.Millisecond)
		pool = &domain.IqiyiPool{VID: vid, CreateTime: now, UpdateTime: now}
		if err := tx.QueryRow(ctx, `INSERT INTO "IqiyiDanmakuPool" ("VID","CreateTime","UpdateTime") VALUES ($1,$2,$2) RETURNING "Id"`, vid, now).Scan(&pool.ID); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return pool, nil
}

func (p *Postgres) ClaimIqiyiPoolSync(ctx context.Context, poolID int, interval time.Duration, force bool) (bool, error) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	tag, err := p.pool.Exec(ctx, `UPDATE "IqiyiDanmakuPool" SET "LastAttemptTime"=$2,"UpdateTime"=$2 WHERE "Id"=$1 AND ($3 OR "LastAttemptTime" IS NULL OR "LastAttemptTime"<=$4)`, poolID, now, force, now.Add(-interval))
	return tag.RowsAffected() > 0, err
}

func (p *Postgres) MergeIqiyiDanmaku(ctx context.Context, poolID int, data []domain.DanmakuData) (int, error) {
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
			timeMillis := int64(math.Round(float64(item.Time) * 1000))
			batch.Queue(`INSERT INTO "IqiyiDanmaku" ("PoolId","TimeMillis","Content","ContentHash","Data","IsBlocked","CreateTime","UpdateTime") VALUES ($1,$2,$3,$4,$5,FALSE,$6,$6) ON CONFLICT ("PoolId","TimeMillis","ContentHash") DO NOTHING`, poolID, timeMillis, content, hex.EncodeToString(hash[:]), raw, now)
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
	if _, err := tx.Exec(ctx, `UPDATE "IqiyiDanmakuPool" SET "LastSyncTime"=$2,"UpdateTime"=$2 WHERE "Id"=$1`, poolID, now); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	p.invalidateDanmakuCache(ctx, "iqiyi")
	return inserted, nil
}

func (p *Postgres) IqiyiPoolData(ctx context.Context, poolID int) ([]domain.DanmakuData, error) {
	return p.cachedDanmaku(ctx, "iqiyi", fmt.Sprint(poolID), func(ctx context.Context) ([]domain.DanmakuData, error) {
		return p.iqiyiPoolDataFromPostgres(ctx, poolID)
	})
}

func (p *Postgres) iqiyiPoolDataFromPostgres(ctx context.Context, poolID int) ([]domain.DanmakuData, error) {
	rows, err := p.pool.Query(ctx, `SELECT d."Data" FROM "IqiyiDanmaku" d WHERE d."PoolId"=$1 AND NOT d."IsBlocked" AND NOT `+iqiyiKeywordBlockedSQL+` ORDER BY d."TimeMillis",d."Id"`, poolID)
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

func (p *Postgres) IqiyiPools(ctx context.Context, filter IqiyiPoolFilter) (domain.Page[domain.IqiyiPool], error) {
	filter.Page, filter.Size = normalizePage(filter.Page, filter.Size)
	where := ""
	args := []any{}
	if strings.TrimSpace(filter.Query) != "" {
		args = append(args, strings.TrimSpace(filter.Query))
		where = ` WHERE p."VID" ILIKE '%' || $1 || '%'`
	}
	var total int
	if err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM "IqiyiDanmakuPool" p`+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.IqiyiPool]{}, err
	}
	args = append(args, filter.Size, filter.Size*(filter.Page-1))
	query := `SELECT p."Id",p."VID",p."LastAttemptTime",p."LastSyncTime",p."CreateTime",p."UpdateTime",
		(SELECT COUNT(*)::integer FROM "IqiyiDanmaku" d WHERE d."PoolId"=p."Id"),
		(SELECT COUNT(*)::integer FROM "IqiyiDanmaku" d WHERE d."PoolId"=p."Id" AND (d."IsBlocked" OR ` + iqiyiKeywordBlockedSQL + `)),
		(SELECT COUNT(*)::integer FROM "IqiyiDanmakuBinding" b WHERE b."PoolId"=p."Id")
		FROM "IqiyiDanmakuPool" p` + where + ` ORDER BY p."VID" LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return domain.Page[domain.IqiyiPool]{}, err
	}
	defer rows.Close()
	list := make([]domain.IqiyiPool, 0)
	for rows.Next() {
		var item domain.IqiyiPool
		if err := rows.Scan(&item.ID, &item.VID, &item.LastAttemptTime, &item.LastSyncTime, &item.CreateTime, &item.UpdateTime, &item.DanmakuCount, &item.BlockedCount, &item.BindingCount); err != nil {
			return domain.Page[domain.IqiyiPool]{}, err
		}
		list = append(list, item)
	}
	return domain.Page[domain.IqiyiPool]{Total: total, List: list}, rows.Err()
}

func (p *Postgres) IqiyiDanmaku(ctx context.Context, filter IqiyiDanmakuFilter) (domain.Page[domain.IqiyiDanmaku], error) {
	filter.Page, filter.Size = normalizePage(filter.Page, filter.Size)
	blocked := `(d."IsBlocked" OR ` + iqiyiKeywordBlockedSQL + `)`
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
	if err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM "IqiyiDanmaku" d`+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.IqiyiDanmaku]{}, err
	}
	args = append(args, filter.Size, filter.Size*(filter.Page-1))
	query := `SELECT d."Id",d."PoolId",d."Data",` + blocked + `,d."IsBlocked",d."CreateTime",d."UpdateTime" FROM "IqiyiDanmaku" d` + where + ` ORDER BY d."TimeMillis" DESC,d."Id" DESC LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return domain.Page[domain.IqiyiDanmaku]{}, err
	}
	defer rows.Close()
	list := make([]domain.IqiyiDanmaku, 0)
	for rows.Next() {
		var item domain.IqiyiDanmaku
		var raw []byte
		if err := rows.Scan(&item.ID, &item.PoolID, &raw, &item.IsBlocked, &item.ManuallyBlocked, &item.CreateTime, &item.UpdateTime); err != nil {
			return domain.Page[domain.IqiyiDanmaku]{}, err
		}
		if err := json.Unmarshal(raw, &item.Data); err != nil {
			return domain.Page[domain.IqiyiDanmaku]{}, err
		}
		list = append(list, item)
	}
	return domain.Page[domain.IqiyiDanmaku]{Total: total, List: list}, rows.Err()
}

func (p *Postgres) SetIqiyiDanmakuBlocked(ctx context.Context, id int64, blocked bool) (bool, error) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	tag, err := p.pool.Exec(ctx, `UPDATE "IqiyiDanmaku" SET "IsBlocked"=$2,"UpdateTime"=$3 WHERE "Id"=$1`, id, blocked, now)
	ok := tag.RowsAffected() > 0
	if err == nil && ok {
		p.invalidateDanmakuCache(ctx, "iqiyi")
	}
	return ok, err
}

func (p *Postgres) IqiyiKeywords(ctx context.Context) ([]domain.IqiyiKeyword, error) {
	rows, err := p.pool.Query(ctx, `SELECT k."Id",k."PoolId",COALESCE(p."VID",''),k."Keyword",k."CreateTime" FROM "IqiyiDanmakuKeyword" k LEFT JOIN "IqiyiDanmakuPool" p ON p."Id"=k."PoolId" ORDER BY k."PoolId" NULLS FIRST,k."Keyword"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.IqiyiKeyword, 0)
	for rows.Next() {
		var item domain.IqiyiKeyword
		if err := rows.Scan(&item.ID, &item.PoolID, &item.PoolVID, &item.Keyword, &item.CreateTime); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (p *Postgres) CreateIqiyiKeyword(ctx context.Context, poolID *int, keyword string) (*domain.IqiyiKeyword, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, fmt.Errorf("keyword cannot be empty")
	}
	hash := sha256.Sum256([]byte(strings.ToLower(keyword)))
	now := time.Now().UTC().Truncate(time.Millisecond)
	var poolValue any
	if poolID != nil {
		if *poolID < 1 {
			return nil, fmt.Errorf("invalid iQiyi pool")
		}
		poolValue = *poolID
	}
	var item domain.IqiyiKeyword
	err := p.pool.QueryRow(ctx, `INSERT INTO "IqiyiDanmakuKeyword" ("PoolId","Keyword","KeywordHash","CreateTime") VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING RETURNING "Id","PoolId","Keyword","CreateTime"`, poolValue, keyword, hex.EncodeToString(hash[:]), now).Scan(&item.ID, &item.PoolID, &item.Keyword, &item.CreateTime)
	if errors.Is(err, pgx.ErrNoRows) {
		err = p.pool.QueryRow(ctx, `SELECT "Id","PoolId","Keyword","CreateTime" FROM "IqiyiDanmakuKeyword" WHERE "PoolId" IS NOT DISTINCT FROM $1::integer AND "KeywordHash"=$2`, poolValue, hex.EncodeToString(hash[:])).Scan(&item.ID, &item.PoolID, &item.Keyword, &item.CreateTime)
	}
	if err != nil {
		return nil, err
	}
	if item.PoolID != nil {
		pool, err := p.IqiyiPool(ctx, *item.PoolID)
		if err != nil {
			return nil, err
		}
		item.PoolVID = pool.VID
	}
	p.invalidateDanmakuCache(ctx, "iqiyi")
	return &item, nil
}

func (p *Postgres) DeleteIqiyiKeyword(ctx context.Context, id int) (bool, error) {
	tag, err := p.pool.Exec(ctx, `DELETE FROM "IqiyiDanmakuKeyword" WHERE "Id"=$1`, id)
	ok := tag.RowsAffected() > 0
	if err == nil && ok {
		p.invalidateDanmakuCache(ctx, "iqiyi")
	}
	return ok, err
}

const iqiyiBindingSelect = `SELECT b."Id",b."Vid",b."PoolId",p."VID",b."Offset",b."CreateTime",b."UpdateTime" FROM "IqiyiDanmakuBinding" b JOIN "IqiyiDanmakuPool" p ON p."Id"=b."PoolId"`

func (p *Postgres) IqiyiBindingsByVID(ctx context.Context, vid string) ([]domain.IqiyiBinding, error) {
	rows, err := p.pool.Query(ctx, iqiyiBindingSelect+` WHERE b."Vid"=$1 ORDER BY b."Id"`, vid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIqiyiBindings(rows)
}

func (p *Postgres) VideoIqiyiBindings(ctx context.Context, videoID int) ([]domain.IqiyiBinding, error) {
	rows, err := p.pool.Query(ctx, iqiyiBindingSelect+` JOIN "Video" v ON v."Vid"=b."Vid" WHERE v."Id"=$1 ORDER BY b."Id"`, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIqiyiBindings(rows)
}

func (p *Postgres) UpsertVideoIqiyiBinding(ctx context.Context, videoID, poolID int, offset float64) (*domain.IqiyiBinding, error) {
	var vid string
	var deleted bool
	if err := p.pool.QueryRow(ctx, `SELECT "Vid","IsDelete" FROM "Video" WHERE "Id"=$1`, videoID).Scan(&vid, &deleted); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if deleted {
		return nil, ErrVideoDeleted
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	var id int
	if err := p.pool.QueryRow(ctx, `INSERT INTO "IqiyiDanmakuBinding" ("Vid","PoolId","Offset","CreateTime","UpdateTime") VALUES ($1,$2,$3,$4,$4) ON CONFLICT ("Vid","PoolId") DO UPDATE SET "Offset"=EXCLUDED."Offset","UpdateTime"=EXCLUDED."UpdateTime" RETURNING "Id"`, vid, poolID, offset, now).Scan(&id); err != nil {
		return nil, err
	}
	return scanIqiyiBinding(p.pool.QueryRow(ctx, iqiyiBindingSelect+` WHERE b."Id"=$1`, id))
}

func (p *Postgres) DeleteVideoIqiyiBinding(ctx context.Context, videoID, bindingID int) (bool, error) {
	tag, err := p.pool.Exec(ctx, `DELETE FROM "IqiyiDanmakuBinding" b USING "Video" v WHERE b."Id"=$1 AND v."Id"=$2 AND b."Vid"=v."Vid"`, bindingID, videoID)
	return tag.RowsAffected() > 0, err
}

func scanIqiyiPool(row pgx.Row) (*domain.IqiyiPool, error) {
	var item domain.IqiyiPool
	err := row.Scan(&item.ID, &item.VID, &item.LastAttemptTime, &item.LastSyncTime, &item.CreateTime, &item.UpdateTime)
	return &item, err
}

func scanIqiyiBinding(row pgx.Row) (*domain.IqiyiBinding, error) {
	var item domain.IqiyiBinding
	err := row.Scan(&item.ID, &item.Vid, &item.PoolID, &item.PoolVID, &item.Offset, &item.CreateTime, &item.UpdateTime)
	return &item, err
}

func scanIqiyiBindings(rows pgx.Rows) ([]domain.IqiyiBinding, error) {
	result := make([]domain.IqiyiBinding, 0)
	for rows.Next() {
		item, err := scanIqiyiBinding(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	return result, rows.Err()
}

func iqiyiPoolAdvisoryKey(vid string) int64 {
	hash := sha256.New()
	var size [8]byte
	for _, value := range []string{"danmaku:iqiyi-pool:v1", vid} {
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write([]byte(value))
	}
	return int64(binary.BigEndian.Uint64(hash.Sum(nil)[:8]))
}
