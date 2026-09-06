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

type AnimekoPoolFilter = BilibiliPoolFilter
type AnimekoDanmakuFilter = BilibiliDanmakuFilter

type AnimekoRepository interface {
	AnimekoPool(context.Context, int) (*domain.AnimekoPool, error)
	EnsureAnimekoPool(context.Context, string) (*domain.AnimekoPool, error)
	ClaimAnimekoPoolSync(context.Context, int, time.Duration, bool) (bool, error)
	MergeAnimekoDanmaku(context.Context, int, []domain.DanmakuData) (int, error)
	AnimekoPoolData(context.Context, int) ([]domain.DanmakuData, error)
	AnimekoPools(context.Context, AnimekoPoolFilter) (domain.Page[domain.AnimekoPool], error)
	AnimekoDanmaku(context.Context, AnimekoDanmakuFilter) (domain.Page[domain.AnimekoDanmaku], error)
	SetAnimekoDanmakuBlocked(context.Context, int64, bool) (bool, error)
	AnimekoKeywords(context.Context) ([]domain.AnimekoKeyword, error)
	CreateAnimekoKeyword(context.Context, *int, string) (*domain.AnimekoKeyword, error)
	DeleteAnimekoKeyword(context.Context, int) (bool, error)
	AnimekoBindingsByVID(context.Context, string) ([]domain.AnimekoBinding, error)
	VideoAnimekoBindings(context.Context, int) ([]domain.AnimekoBinding, error)
	UpsertVideoAnimekoBinding(context.Context, int, int, float64) (*domain.AnimekoBinding, error)
	DeleteVideoAnimekoBinding(context.Context, int, int) (bool, error)
}

const animekoKeywordBlockedSQL = `EXISTS (
	SELECT 1
	FROM "AnimekoDanmakuKeyword" k
	WHERE (k."PoolId" IS NULL OR k."PoolId"=d."PoolId")
		AND strpos(lower(d."Content"), lower(k."Keyword"))>0
)`

const animekoPoolSelect = `SELECT "Id","EpisodeId","LastAttemptTime","LastSyncTime","CreateTime","UpdateTime" FROM "AnimekoDanmakuPool"`

func (p *Postgres) AnimekoPool(ctx context.Context, id int) (*domain.AnimekoPool, error) {
	pool, err := scanAnimekoPool(p.pool.QueryRow(ctx, animekoPoolSelect+` WHERE "Id"=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return pool, err
}

func (p *Postgres) EnsureAnimekoPool(ctx context.Context, vid string) (*domain.AnimekoPool, error) {
	vid = strings.TrimSpace(vid)
	id, parseErr := strconv.ParseInt(vid, 10, 64)
	if parseErr != nil || id <= 0 || strings.HasPrefix(vid, "+") {
		return nil, fmt.Errorf("invalid animeko pool identity")
	}
	vid = strconv.FormatInt(id, 10)
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, animekoPoolAdvisoryKey(vid)); err != nil {
		return nil, err
	}
	pool, err := scanAnimekoPool(tx.QueryRow(ctx, animekoPoolSelect+` WHERE "EpisodeId"=$1`, vid))
	if errors.Is(err, pgx.ErrNoRows) {
		now := time.Now().UTC().Truncate(time.Millisecond)
		pool = &domain.AnimekoPool{EpisodeID: vid, CreateTime: now, UpdateTime: now}
		if err := tx.QueryRow(ctx, `INSERT INTO "AnimekoDanmakuPool" ("EpisodeId","CreateTime","UpdateTime") VALUES ($1,$2,$2) RETURNING "Id"`, vid, now).Scan(&pool.ID); err != nil {
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

func (p *Postgres) ClaimAnimekoPoolSync(ctx context.Context, poolID int, interval time.Duration, force bool) (bool, error) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	tag, err := p.pool.Exec(ctx, `UPDATE "AnimekoDanmakuPool" SET "LastAttemptTime"=$2,"UpdateTime"=$2 WHERE "Id"=$1 AND ($3 OR "LastAttemptTime" IS NULL OR "LastAttemptTime"<=$4)`, poolID, now, force, now.Add(-interval))
	return tag.RowsAffected() > 0, err
}

func (p *Postgres) MergeAnimekoDanmaku(ctx context.Context, poolID int, data []domain.DanmakuData) (int, error) {
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
			batch.Queue(`INSERT INTO "AnimekoDanmaku" ("PoolId","TimeMillis","Content","ContentHash","Data","IsBlocked","CreateTime","UpdateTime") VALUES ($1,$2,$3,$4,$5,FALSE,$6,$6) ON CONFLICT ("PoolId","TimeMillis","ContentHash") DO NOTHING`, poolID, timeMillis, content, hex.EncodeToString(hash[:]), raw, now)
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
	if _, err := tx.Exec(ctx, `UPDATE "AnimekoDanmakuPool" SET "LastSyncTime"=$2,"UpdateTime"=$2 WHERE "Id"=$1`, poolID, now); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	p.invalidateDanmakuCache(ctx, "animeko")
	return inserted, nil
}

func (p *Postgres) AnimekoPoolData(ctx context.Context, poolID int) ([]domain.DanmakuData, error) {
	return p.cachedDanmaku(ctx, "animeko", fmt.Sprint(poolID), func(ctx context.Context) ([]domain.DanmakuData, error) {
		return p.animekoPoolDataFromPostgres(ctx, poolID)
	})
}

func (p *Postgres) animekoPoolDataFromPostgres(ctx context.Context, poolID int) ([]domain.DanmakuData, error) {
	rows, err := p.pool.Query(ctx, `SELECT d."Data" FROM "AnimekoDanmaku" d WHERE d."PoolId"=$1 AND NOT d."IsBlocked" AND NOT `+animekoKeywordBlockedSQL+` ORDER BY d."TimeMillis",d."Id"`, poolID)
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

func (p *Postgres) AnimekoPools(ctx context.Context, filter AnimekoPoolFilter) (domain.Page[domain.AnimekoPool], error) {
	filter.Page, filter.Size = normalizePage(filter.Page, filter.Size)
	where := ""
	args := []any{}
	if strings.TrimSpace(filter.Query) != "" {
		args = append(args, strings.TrimSpace(filter.Query))
		where = ` WHERE p."EpisodeId" ILIKE '%' || $1 || '%'`
	}
	var total int
	if err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM "AnimekoDanmakuPool" p`+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.AnimekoPool]{}, err
	}
	args = append(args, filter.Size, filter.Size*(filter.Page-1))
	query := `SELECT p."Id",p."EpisodeId",p."LastAttemptTime",p."LastSyncTime",p."CreateTime",p."UpdateTime",
		(SELECT COUNT(*)::integer FROM "AnimekoDanmaku" d WHERE d."PoolId"=p."Id"),
		(SELECT COUNT(*)::integer FROM "AnimekoDanmaku" d WHERE d."PoolId"=p."Id" AND (d."IsBlocked" OR ` + animekoKeywordBlockedSQL + `)),
		(SELECT COUNT(*)::integer FROM "AnimekoDanmakuBinding" b WHERE b."PoolId"=p."Id")
		FROM "AnimekoDanmakuPool" p` + where + ` ORDER BY p."EpisodeId" LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return domain.Page[domain.AnimekoPool]{}, err
	}
	defer rows.Close()
	list := make([]domain.AnimekoPool, 0)
	for rows.Next() {
		var item domain.AnimekoPool
		if err := rows.Scan(&item.ID, &item.EpisodeID, &item.LastAttemptTime, &item.LastSyncTime, &item.CreateTime, &item.UpdateTime, &item.DanmakuCount, &item.BlockedCount, &item.BindingCount); err != nil {
			return domain.Page[domain.AnimekoPool]{}, err
		}
		list = append(list, item)
	}
	return domain.Page[domain.AnimekoPool]{Total: total, List: list}, rows.Err()
}

func (p *Postgres) AnimekoDanmaku(ctx context.Context, filter AnimekoDanmakuFilter) (domain.Page[domain.AnimekoDanmaku], error) {
	filter.Page, filter.Size = normalizePage(filter.Page, filter.Size)
	blocked := `(d."IsBlocked" OR ` + animekoKeywordBlockedSQL + `)`
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
	if err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM "AnimekoDanmaku" d`+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.AnimekoDanmaku]{}, err
	}
	args = append(args, filter.Size, filter.Size*(filter.Page-1))
	query := `SELECT d."Id",d."PoolId",d."Data",` + blocked + `,d."IsBlocked",d."CreateTime",d."UpdateTime" FROM "AnimekoDanmaku" d` + where + ` ORDER BY d."TimeMillis" DESC,d."Id" DESC LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return domain.Page[domain.AnimekoDanmaku]{}, err
	}
	defer rows.Close()
	list := make([]domain.AnimekoDanmaku, 0)
	for rows.Next() {
		var item domain.AnimekoDanmaku
		var raw []byte
		if err := rows.Scan(&item.ID, &item.PoolID, &raw, &item.IsBlocked, &item.ManuallyBlocked, &item.CreateTime, &item.UpdateTime); err != nil {
			return domain.Page[domain.AnimekoDanmaku]{}, err
		}
		if err := json.Unmarshal(raw, &item.Data); err != nil {
			return domain.Page[domain.AnimekoDanmaku]{}, err
		}
		list = append(list, item)
	}
	return domain.Page[domain.AnimekoDanmaku]{Total: total, List: list}, rows.Err()
}

func (p *Postgres) SetAnimekoDanmakuBlocked(ctx context.Context, id int64, blocked bool) (bool, error) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	tag, err := p.pool.Exec(ctx, `UPDATE "AnimekoDanmaku" SET "IsBlocked"=$2,"UpdateTime"=$3 WHERE "Id"=$1`, id, blocked, now)
	ok := tag.RowsAffected() > 0
	if err == nil && ok {
		p.invalidateDanmakuCache(ctx, "animeko")
	}
	return ok, err
}

func (p *Postgres) AnimekoKeywords(ctx context.Context) ([]domain.AnimekoKeyword, error) {
	rows, err := p.pool.Query(ctx, `SELECT k."Id",k."PoolId",COALESCE(p."EpisodeId",''),k."Keyword",k."CreateTime" FROM "AnimekoDanmakuKeyword" k LEFT JOIN "AnimekoDanmakuPool" p ON p."Id"=k."PoolId" ORDER BY k."PoolId" NULLS FIRST,k."Keyword"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.AnimekoKeyword, 0)
	for rows.Next() {
		var item domain.AnimekoKeyword
		if err := rows.Scan(&item.ID, &item.PoolID, &item.PoolEpisodeID, &item.Keyword, &item.CreateTime); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (p *Postgres) CreateAnimekoKeyword(ctx context.Context, poolID *int, keyword string) (*domain.AnimekoKeyword, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, fmt.Errorf("keyword cannot be empty")
	}
	hash := sha256.Sum256([]byte(strings.ToLower(keyword)))
	now := time.Now().UTC().Truncate(time.Millisecond)
	var poolValue any
	if poolID != nil {
		if *poolID < 1 {
			return nil, fmt.Errorf("invalid animeko pool")
		}
		poolValue = *poolID
	}
	var item domain.AnimekoKeyword
	err := p.pool.QueryRow(ctx, `INSERT INTO "AnimekoDanmakuKeyword" ("PoolId","Keyword","KeywordHash","CreateTime") VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING RETURNING "Id","PoolId","Keyword","CreateTime"`, poolValue, keyword, hex.EncodeToString(hash[:]), now).Scan(&item.ID, &item.PoolID, &item.Keyword, &item.CreateTime)
	if errors.Is(err, pgx.ErrNoRows) {
		err = p.pool.QueryRow(ctx, `SELECT "Id","PoolId","Keyword","CreateTime" FROM "AnimekoDanmakuKeyword" WHERE "PoolId" IS NOT DISTINCT FROM $1::integer AND "KeywordHash"=$2`, poolValue, hex.EncodeToString(hash[:])).Scan(&item.ID, &item.PoolID, &item.Keyword, &item.CreateTime)
	}
	if err != nil {
		return nil, err
	}
	if item.PoolID != nil {
		pool, err := p.AnimekoPool(ctx, *item.PoolID)
		if err != nil {
			return nil, err
		}
		item.PoolEpisodeID = pool.EpisodeID
	}
	p.invalidateDanmakuCache(ctx, "animeko")
	return &item, nil
}

func (p *Postgres) DeleteAnimekoKeyword(ctx context.Context, id int) (bool, error) {
	tag, err := p.pool.Exec(ctx, `DELETE FROM "AnimekoDanmakuKeyword" WHERE "Id"=$1`, id)
	ok := tag.RowsAffected() > 0
	if err == nil && ok {
		p.invalidateDanmakuCache(ctx, "animeko")
	}
	return ok, err
}

const animekoBindingSelect = `SELECT b."Id",b."Vid",b."PoolId",p."EpisodeId",b."Offset",b."CreateTime",b."UpdateTime" FROM "AnimekoDanmakuBinding" b JOIN "AnimekoDanmakuPool" p ON p."Id"=b."PoolId"`

func (p *Postgres) AnimekoBindingsByVID(ctx context.Context, vid string) ([]domain.AnimekoBinding, error) {
	rows, err := p.pool.Query(ctx, animekoBindingSelect+` WHERE b."Vid"=$1 ORDER BY b."Id"`, vid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAnimekoBindings(rows)
}

func (p *Postgres) VideoAnimekoBindings(ctx context.Context, videoID int) ([]domain.AnimekoBinding, error) {
	rows, err := p.pool.Query(ctx, animekoBindingSelect+` JOIN "Video" v ON v."Vid"=b."Vid" WHERE v."Id"=$1 ORDER BY b."Id"`, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAnimekoBindings(rows)
}

func (p *Postgres) UpsertVideoAnimekoBinding(ctx context.Context, videoID, poolID int, offset float64) (*domain.AnimekoBinding, error) {
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
	if err := p.pool.QueryRow(ctx, `INSERT INTO "AnimekoDanmakuBinding" ("Vid","PoolId","Offset","CreateTime","UpdateTime") VALUES ($1,$2,$3,$4,$4) ON CONFLICT ("Vid","PoolId") DO UPDATE SET "Offset"=EXCLUDED."Offset","UpdateTime"=EXCLUDED."UpdateTime" RETURNING "Id"`, vid, poolID, offset, now).Scan(&id); err != nil {
		return nil, err
	}
	return scanAnimekoBinding(p.pool.QueryRow(ctx, animekoBindingSelect+` WHERE b."Id"=$1`, id))
}

func (p *Postgres) DeleteVideoAnimekoBinding(ctx context.Context, videoID, bindingID int) (bool, error) {
	tag, err := p.pool.Exec(ctx, `DELETE FROM "AnimekoDanmakuBinding" b USING "Video" v WHERE b."Id"=$1 AND v."Id"=$2 AND b."Vid"=v."Vid"`, bindingID, videoID)
	return tag.RowsAffected() > 0, err
}

func scanAnimekoPool(row pgx.Row) (*domain.AnimekoPool, error) {
	var item domain.AnimekoPool
	err := row.Scan(&item.ID, &item.EpisodeID, &item.LastAttemptTime, &item.LastSyncTime, &item.CreateTime, &item.UpdateTime)
	return &item, err
}

func scanAnimekoBinding(row pgx.Row) (*domain.AnimekoBinding, error) {
	var item domain.AnimekoBinding
	err := row.Scan(&item.ID, &item.Vid, &item.PoolID, &item.PoolEpisodeID, &item.Offset, &item.CreateTime, &item.UpdateTime)
	return &item, err
}

func scanAnimekoBindings(rows pgx.Rows) ([]domain.AnimekoBinding, error) {
	result := make([]domain.AnimekoBinding, 0)
	for rows.Next() {
		item, err := scanAnimekoBinding(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	return result, rows.Err()
}

func animekoPoolAdvisoryKey(vid string) int64 {
	hash := sha256.New()
	var size [8]byte
	for _, value := range []string{"danmaku:animeko-pool:v1", vid} {
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write([]byte(value))
	}
	return int64(binary.BigEndian.Uint64(hash.Sum(nil)[:8]))
}
