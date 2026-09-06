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
	"strconv"
	"strings"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"github.com/jackc/pgx/v5"
)

type DandanplayPoolFilter = BilibiliPoolFilter
type DandanplayDanmakuFilter = BilibiliDanmakuFilter

type DandanplayRepository interface {
	DandanplayPool(context.Context, int) (*domain.DandanplayPool, error)
	EnsureDandanplayPool(context.Context, string) (*domain.DandanplayPool, error)
	ClaimDandanplayPoolSync(context.Context, int, time.Duration, bool) (bool, error)
	MergeDandanplayDanmaku(context.Context, int, []domain.DanmakuData) (int, error)
	DandanplayPoolData(context.Context, int) ([]domain.DanmakuData, error)
	DandanplayPools(context.Context, DandanplayPoolFilter) (domain.Page[domain.DandanplayPool], error)
	DandanplayDanmaku(context.Context, DandanplayDanmakuFilter) (domain.Page[domain.DandanplayDanmaku], error)
	SetDandanplayDanmakuBlocked(context.Context, int64, bool) (bool, error)
	DandanplayKeywords(context.Context) ([]domain.DandanplayKeyword, error)
	CreateDandanplayKeyword(context.Context, *int, string) (*domain.DandanplayKeyword, error)
	DeleteDandanplayKeyword(context.Context, int) (bool, error)
	DandanplayBindingsByVID(context.Context, string) ([]domain.DandanplayBinding, error)
	VideoDandanplayBindings(context.Context, int) ([]domain.DandanplayBinding, error)
	UpsertVideoDandanplayBinding(context.Context, int, int, float64) (*domain.DandanplayBinding, error)
	DeleteVideoDandanplayBinding(context.Context, int, int) (bool, error)
}

const dandanplayKeywordBlockedSQL = `EXISTS (
	SELECT 1
	FROM "DandanplayDanmakuKeyword" k
	WHERE (k."PoolId" IS NULL OR k."PoolId"=d."PoolId")
		AND strpos(lower(d."Content"), lower(k."Keyword"))>0
)`

const dandanplayPoolSelect = `SELECT "Id","EpisodeId","LastAttemptTime","LastSyncTime","CreateTime","UpdateTime" FROM "DandanplayDanmakuPool"`

func (p *Postgres) DandanplayPool(ctx context.Context, id int) (*domain.DandanplayPool, error) {
	pool, err := scanDandanplayPool(p.pool.QueryRow(ctx, dandanplayPoolSelect+` WHERE "Id"=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return pool, err
}

func (p *Postgres) EnsureDandanplayPool(ctx context.Context, vid string) (*domain.DandanplayPool, error) {
	vid = strings.TrimSpace(vid)
	id, parseErr := strconv.ParseInt(vid, 10, 64)
	if parseErr != nil || id <= 0 || strings.HasPrefix(vid, "+") {
		return nil, fmt.Errorf("invalid dandanplay pool identity")
	}
	vid = strconv.FormatInt(id, 10)
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, dandanplayPoolAdvisoryKey(vid)); err != nil {
		return nil, err
	}
	pool, err := scanDandanplayPool(tx.QueryRow(ctx, dandanplayPoolSelect+` WHERE "EpisodeId"=$1`, vid))
	if errors.Is(err, pgx.ErrNoRows) {
		now := time.Now().UTC().Truncate(time.Millisecond)
		pool = &domain.DandanplayPool{EpisodeID: vid, CreateTime: now, UpdateTime: now}
		if err := tx.QueryRow(ctx, `INSERT INTO "DandanplayDanmakuPool" ("EpisodeId","CreateTime","UpdateTime") VALUES ($1,$2,$2) RETURNING "Id"`, vid, now).Scan(&pool.ID); err != nil {
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

func (p *Postgres) ClaimDandanplayPoolSync(ctx context.Context, poolID int, interval time.Duration, force bool) (bool, error) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	tag, err := p.pool.Exec(ctx, `UPDATE "DandanplayDanmakuPool" SET "LastAttemptTime"=$2,"UpdateTime"=$2 WHERE "Id"=$1 AND ($3 OR "LastAttemptTime" IS NULL OR "LastAttemptTime"<=$4)`, poolID, now, force, now.Add(-interval))
	return tag.RowsAffected() > 0, err
}

func (p *Postgres) MergeDandanplayDanmaku(ctx context.Context, poolID int, data []domain.DanmakuData) (int, error) {
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
			batch.Queue(`INSERT INTO "DandanplayDanmaku" ("PoolId","TimeMillis","Content","ContentHash","Data","IsBlocked","CreateTime","UpdateTime") VALUES ($1,$2,$3,$4,$5,FALSE,$6,$6) ON CONFLICT ("PoolId","TimeMillis","ContentHash") DO NOTHING`, poolID, timeMillis, content, hex.EncodeToString(hash[:]), raw, now)
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
	if _, err := tx.Exec(ctx, `UPDATE "DandanplayDanmakuPool" SET "LastSyncTime"=$2,"UpdateTime"=$2 WHERE "Id"=$1`, poolID, now); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	p.invalidateDanmakuCache(ctx, "dandanplay")
	return inserted, nil
}

func (p *Postgres) DandanplayPoolData(ctx context.Context, poolID int) ([]domain.DanmakuData, error) {
	return p.cachedDanmaku(ctx, "dandanplay", fmt.Sprint(poolID), func(ctx context.Context) ([]domain.DanmakuData, error) {
		return p.dandanplayPoolDataFromPostgres(ctx, poolID)
	})
}

func (p *Postgres) dandanplayPoolDataFromPostgres(ctx context.Context, poolID int) ([]domain.DanmakuData, error) {
	rows, err := p.pool.Query(ctx, `SELECT d."Data" FROM "DandanplayDanmaku" d WHERE d."PoolId"=$1 AND NOT d."IsBlocked" AND NOT `+dandanplayKeywordBlockedSQL+` ORDER BY d."TimeMillis",d."Id"`, poolID)
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

func (p *Postgres) DandanplayPools(ctx context.Context, filter DandanplayPoolFilter) (domain.Page[domain.DandanplayPool], error) {
	filter.Page, filter.Size = normalizePage(filter.Page, filter.Size)
	where := ""
	args := []any{}
	if strings.TrimSpace(filter.Query) != "" {
		args = append(args, strings.TrimSpace(filter.Query))
		where = ` WHERE p."EpisodeId" ILIKE '%' || $1 || '%'`
	}
	var total int
	if err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM "DandanplayDanmakuPool" p`+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.DandanplayPool]{}, err
	}
	args = append(args, filter.Size, filter.Size*(filter.Page-1))
	query := `SELECT p."Id",p."EpisodeId",p."LastAttemptTime",p."LastSyncTime",p."CreateTime",p."UpdateTime",
		(SELECT COUNT(*)::integer FROM "DandanplayDanmaku" d WHERE d."PoolId"=p."Id"),
		(SELECT COUNT(*)::integer FROM "DandanplayDanmaku" d WHERE d."PoolId"=p."Id" AND (d."IsBlocked" OR ` + dandanplayKeywordBlockedSQL + `)),
		(SELECT COUNT(*)::integer FROM "DandanplayDanmakuBinding" b WHERE b."PoolId"=p."Id")
		FROM "DandanplayDanmakuPool" p` + where + ` ORDER BY p."EpisodeId" LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return domain.Page[domain.DandanplayPool]{}, err
	}
	defer rows.Close()
	list := make([]domain.DandanplayPool, 0)
	for rows.Next() {
		var item domain.DandanplayPool
		if err := rows.Scan(&item.ID, &item.EpisodeID, &item.LastAttemptTime, &item.LastSyncTime, &item.CreateTime, &item.UpdateTime, &item.DanmakuCount, &item.BlockedCount, &item.BindingCount); err != nil {
			return domain.Page[domain.DandanplayPool]{}, err
		}
		list = append(list, item)
	}
	return domain.Page[domain.DandanplayPool]{Total: total, List: list}, rows.Err()
}

func (p *Postgres) DandanplayDanmaku(ctx context.Context, filter DandanplayDanmakuFilter) (domain.Page[domain.DandanplayDanmaku], error) {
	filter.Page, filter.Size = normalizePage(filter.Page, filter.Size)
	blocked := `(d."IsBlocked" OR ` + dandanplayKeywordBlockedSQL + `)`
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
	if err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM "DandanplayDanmaku" d`+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.DandanplayDanmaku]{}, err
	}
	args = append(args, filter.Size, filter.Size*(filter.Page-1))
	query := `SELECT d."Id",d."PoolId",d."Data",` + blocked + `,d."IsBlocked",d."CreateTime",d."UpdateTime" FROM "DandanplayDanmaku" d` + where + ` ORDER BY d."TimeMillis" DESC,d."Id" DESC LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return domain.Page[domain.DandanplayDanmaku]{}, err
	}
	defer rows.Close()
	list := make([]domain.DandanplayDanmaku, 0)
	for rows.Next() {
		var item domain.DandanplayDanmaku
		var raw []byte
		if err := rows.Scan(&item.ID, &item.PoolID, &raw, &item.IsBlocked, &item.ManuallyBlocked, &item.CreateTime, &item.UpdateTime); err != nil {
			return domain.Page[domain.DandanplayDanmaku]{}, err
		}
		if err := json.Unmarshal(raw, &item.Data); err != nil {
			return domain.Page[domain.DandanplayDanmaku]{}, err
		}
		list = append(list, item)
	}
	return domain.Page[domain.DandanplayDanmaku]{Total: total, List: list}, rows.Err()
}

func (p *Postgres) SetDandanplayDanmakuBlocked(ctx context.Context, id int64, blocked bool) (bool, error) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	tag, err := p.pool.Exec(ctx, `UPDATE "DandanplayDanmaku" SET "IsBlocked"=$2,"UpdateTime"=$3 WHERE "Id"=$1`, id, blocked, now)
	ok := tag.RowsAffected() > 0
	if err == nil && ok {
		p.invalidateDanmakuCache(ctx, "dandanplay")
	}
	return ok, err
}

func (p *Postgres) DandanplayKeywords(ctx context.Context) ([]domain.DandanplayKeyword, error) {
	rows, err := p.pool.Query(ctx, `SELECT k."Id",k."PoolId",COALESCE(p."EpisodeId",''),k."Keyword",k."CreateTime" FROM "DandanplayDanmakuKeyword" k LEFT JOIN "DandanplayDanmakuPool" p ON p."Id"=k."PoolId" ORDER BY k."PoolId" NULLS FIRST,k."Keyword"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.DandanplayKeyword, 0)
	for rows.Next() {
		var item domain.DandanplayKeyword
		if err := rows.Scan(&item.ID, &item.PoolID, &item.PoolEpisodeID, &item.Keyword, &item.CreateTime); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (p *Postgres) CreateDandanplayKeyword(ctx context.Context, poolID *int, keyword string) (*domain.DandanplayKeyword, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, fmt.Errorf("keyword cannot be empty")
	}
	hash := sha256.Sum256([]byte(strings.ToLower(keyword)))
	now := time.Now().UTC().Truncate(time.Millisecond)
	var poolValue any
	if poolID != nil {
		if *poolID < 1 {
			return nil, fmt.Errorf("invalid dandanplay pool")
		}
		poolValue = *poolID
	}
	var item domain.DandanplayKeyword
	err := p.pool.QueryRow(ctx, `INSERT INTO "DandanplayDanmakuKeyword" ("PoolId","Keyword","KeywordHash","CreateTime") VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING RETURNING "Id","PoolId","Keyword","CreateTime"`, poolValue, keyword, hex.EncodeToString(hash[:]), now).Scan(&item.ID, &item.PoolID, &item.Keyword, &item.CreateTime)
	if errors.Is(err, pgx.ErrNoRows) {
		err = p.pool.QueryRow(ctx, `SELECT "Id","PoolId","Keyword","CreateTime" FROM "DandanplayDanmakuKeyword" WHERE "PoolId" IS NOT DISTINCT FROM $1::integer AND "KeywordHash"=$2`, poolValue, hex.EncodeToString(hash[:])).Scan(&item.ID, &item.PoolID, &item.Keyword, &item.CreateTime)
	}
	if err != nil {
		return nil, err
	}
	if item.PoolID != nil {
		pool, err := p.DandanplayPool(ctx, *item.PoolID)
		if err != nil {
			return nil, err
		}
		item.PoolEpisodeID = pool.EpisodeID
	}
	p.invalidateDanmakuCache(ctx, "dandanplay")
	return &item, nil
}

func (p *Postgres) DeleteDandanplayKeyword(ctx context.Context, id int) (bool, error) {
	tag, err := p.pool.Exec(ctx, `DELETE FROM "DandanplayDanmakuKeyword" WHERE "Id"=$1`, id)
	ok := tag.RowsAffected() > 0
	if err == nil && ok {
		p.invalidateDanmakuCache(ctx, "dandanplay")
	}
	return ok, err
}

const dandanplayBindingSelect = `SELECT b."Id",b."Vid",b."PoolId",p."EpisodeId",b."Offset",b."CreateTime",b."UpdateTime" FROM "DandanplayDanmakuBinding" b JOIN "DandanplayDanmakuPool" p ON p."Id"=b."PoolId"`

func (p *Postgres) DandanplayBindingsByVID(ctx context.Context, vid string) ([]domain.DandanplayBinding, error) {
	rows, err := p.pool.Query(ctx, dandanplayBindingSelect+` WHERE b."Vid"=$1 ORDER BY b."Id"`, vid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDandanplayBindings(rows)
}

func (p *Postgres) VideoDandanplayBindings(ctx context.Context, videoID int) ([]domain.DandanplayBinding, error) {
	rows, err := p.pool.Query(ctx, dandanplayBindingSelect+` JOIN "Video" v ON v."Vid"=b."Vid" WHERE v."Id"=$1 ORDER BY b."Id"`, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDandanplayBindings(rows)
}

func (p *Postgres) UpsertVideoDandanplayBinding(ctx context.Context, videoID, poolID int, offset float64) (*domain.DandanplayBinding, error) {
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
	if err := p.pool.QueryRow(ctx, `INSERT INTO "DandanplayDanmakuBinding" ("Vid","PoolId","Offset","CreateTime","UpdateTime") VALUES ($1,$2,$3,$4,$4) ON CONFLICT ("Vid","PoolId") DO UPDATE SET "Offset"=EXCLUDED."Offset","UpdateTime"=EXCLUDED."UpdateTime" RETURNING "Id"`, vid, poolID, offset, now).Scan(&id); err != nil {
		return nil, err
	}
	return scanDandanplayBinding(p.pool.QueryRow(ctx, dandanplayBindingSelect+` WHERE b."Id"=$1`, id))
}

func (p *Postgres) DeleteVideoDandanplayBinding(ctx context.Context, videoID, bindingID int) (bool, error) {
	tag, err := p.pool.Exec(ctx, `DELETE FROM "DandanplayDanmakuBinding" b USING "Video" v WHERE b."Id"=$1 AND v."Id"=$2 AND b."Vid"=v."Vid"`, bindingID, videoID)
	return tag.RowsAffected() > 0, err
}

func scanDandanplayPool(row pgx.Row) (*domain.DandanplayPool, error) {
	var item domain.DandanplayPool
	err := row.Scan(&item.ID, &item.EpisodeID, &item.LastAttemptTime, &item.LastSyncTime, &item.CreateTime, &item.UpdateTime)
	return &item, err
}

func scanDandanplayBinding(row pgx.Row) (*domain.DandanplayBinding, error) {
	var item domain.DandanplayBinding
	err := row.Scan(&item.ID, &item.Vid, &item.PoolID, &item.PoolEpisodeID, &item.Offset, &item.CreateTime, &item.UpdateTime)
	return &item, err
}

func scanDandanplayBindings(rows pgx.Rows) ([]domain.DandanplayBinding, error) {
	result := make([]domain.DandanplayBinding, 0)
	for rows.Next() {
		item, err := scanDandanplayBinding(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	return result, rows.Err()
}

func dandanplayPoolAdvisoryKey(vid string) int64 {
	hash := sha256.New()
	var size [8]byte
	for _, value := range []string{"danmaku:dandanplay-pool:v1", vid} {
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write([]byte(value))
	}
	return int64(binary.BigEndian.Uint64(hash.Sum(nil)[:8]))
}
