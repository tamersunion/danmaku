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

type CatalogPoolFilter = BilibiliPoolFilter
type CatalogDanmakuFilter = BilibiliDanmakuFilter

type CatalogRepository interface {
	CatalogPool(context.Context, int) (*domain.CatalogPool, error)
	EnsureCatalogPool(context.Context, string) (*domain.CatalogPool, error)
	ClaimCatalogPoolSync(context.Context, int, time.Duration, bool) (bool, error)
	MergeCatalogDanmaku(context.Context, int, []domain.DanmakuData) (int, error)
	CatalogPoolData(context.Context, int) ([]domain.DanmakuData, error)
	CatalogPools(context.Context, CatalogPoolFilter) (domain.Page[domain.CatalogPool], error)
	CatalogDanmaku(context.Context, CatalogDanmakuFilter) (domain.Page[domain.CatalogDanmaku], error)
	SetCatalogDanmakuBlocked(context.Context, int64, bool) (bool, error)
	CatalogKeywords(context.Context) ([]domain.CatalogKeyword, error)
	CreateCatalogKeyword(context.Context, *int, string) (*domain.CatalogKeyword, error)
	DeleteCatalogKeyword(context.Context, int) (bool, error)
	CatalogBindingsByVID(context.Context, string) ([]domain.CatalogBinding, error)
	VideoCatalogBindings(context.Context, int) ([]domain.CatalogBinding, error)
	UpsertVideoCatalogBinding(context.Context, int, int, float64) (*domain.CatalogBinding, error)
	DeleteVideoCatalogBinding(context.Context, int, int) (bool, error)
}

const catalogKeywordBlockedSQL = `EXISTS (
	SELECT 1
	FROM "CatalogDanmakuKeyword" k
	WHERE (k."PoolId" IS NULL OR k."PoolId"=d."PoolId")
		AND strpos(lower(d."Content"), lower(k."Keyword"))>0
)`

const catalogPoolSelect = `SELECT "Id","EpisodeId","LastAttemptTime","LastSyncTime","CreateTime","UpdateTime" FROM "CatalogDanmakuPool"`

func (p *CatalogStore) CatalogPool(ctx context.Context, id int) (*domain.CatalogPool, error) {
	pool, err := scanCatalogPool(p.db.QueryRow(ctx, catalogPoolSelect+` WHERE "Id"=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return pool, err
}

func (p *CatalogStore) EnsureCatalogPool(ctx context.Context, vid string) (*domain.CatalogPool, error) {
	vid = strings.TrimSpace(vid)
	if !validCatalogID(vid) {
		return nil, fmt.Errorf("invalid catalog pool identity")
	}
	tx, err := p.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, catalogPoolAdvisoryKey(p.source+":"+vid)); err != nil {
		return nil, err
	}
	pool, err := scanCatalogPool(tx.QueryRow(ctx, catalogPoolSelect+` WHERE "EpisodeId"=$1`, vid))
	if errors.Is(err, pgx.ErrNoRows) {
		now := time.Now().UTC().Truncate(time.Millisecond)
		pool = &domain.CatalogPool{EpisodeID: vid, CreateTime: now, UpdateTime: now}
		if err := tx.QueryRow(ctx, `INSERT INTO "CatalogDanmakuPool" ("EpisodeId","CreateTime","UpdateTime") VALUES ($1,$2,$2) RETURNING "Id"`, vid, now).Scan(&pool.ID); err != nil {
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

func (p *CatalogStore) ClaimCatalogPoolSync(ctx context.Context, poolID int, interval time.Duration, force bool) (bool, error) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	tag, err := p.db.Exec(ctx, `UPDATE "CatalogDanmakuPool" SET "LastAttemptTime"=$2,"UpdateTime"=$2 WHERE "Id"=$1 AND ($3 OR "LastAttemptTime" IS NULL OR "LastAttemptTime"<=$4)`, poolID, now, force, now.Add(-interval))
	return tag.RowsAffected() > 0, err
}

func (p *CatalogStore) MergeCatalogDanmaku(ctx context.Context, poolID int, data []domain.DanmakuData) (int, error) {
	tx, err := p.db.Begin(ctx)
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
			batch.Queue(p.db.sql(`INSERT INTO "CatalogDanmaku" ("PoolId","TimeMillis","Content","ContentHash","Data","IsBlocked","CreateTime","UpdateTime") VALUES ($1,$2,$3,$4,$5,FALSE,$6,$6) ON CONFLICT ("PoolId","TimeMillis","ContentHash") DO NOTHING`), poolID, timeMillis, content, hex.EncodeToString(hash[:]), raw, now)
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
	if _, err := tx.Exec(ctx, `UPDATE "CatalogDanmakuPool" SET "LastSyncTime"=$2,"UpdateTime"=$2 WHERE "Id"=$1`, poolID, now); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	p.invalidateDanmakuCache(ctx, p.source)
	return inserted, nil
}

func (p *CatalogStore) CatalogPoolData(ctx context.Context, poolID int) ([]domain.DanmakuData, error) {
	return p.cachedDanmaku(ctx, p.source, fmt.Sprint(poolID), func(ctx context.Context) ([]domain.DanmakuData, error) {
		return p.catalogPoolDataFromPostgres(ctx, poolID)
	})
}

func (p *CatalogStore) catalogPoolDataFromPostgres(ctx context.Context, poolID int) ([]domain.DanmakuData, error) {
	rows, err := p.db.Query(ctx, `SELECT d."Data" FROM "CatalogDanmaku" d WHERE d."PoolId"=$1 AND NOT d."IsBlocked" AND NOT `+catalogKeywordBlockedSQL+` ORDER BY d."TimeMillis",d."Id"`, poolID)
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

func (p *CatalogStore) CatalogPools(ctx context.Context, filter CatalogPoolFilter) (domain.Page[domain.CatalogPool], error) {
	filter.Page, filter.Size = normalizePage(filter.Page, filter.Size)
	where := ""
	args := []any{}
	if strings.TrimSpace(filter.Query) != "" {
		args = append(args, strings.TrimSpace(filter.Query))
		where = ` WHERE p."EpisodeId" ILIKE '%' || $1 || '%'`
	}
	var total int
	if err := p.db.QueryRow(ctx, `SELECT COUNT(*) FROM "CatalogDanmakuPool" p`+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.CatalogPool]{}, err
	}
	args = append(args, filter.Size, filter.Size*(filter.Page-1))
	query := `SELECT p."Id",p."EpisodeId",p."LastAttemptTime",p."LastSyncTime",p."CreateTime",p."UpdateTime",
		(SELECT COUNT(*)::integer FROM "CatalogDanmaku" d WHERE d."PoolId"=p."Id"),
		(SELECT COUNT(*)::integer FROM "CatalogDanmaku" d WHERE d."PoolId"=p."Id" AND (d."IsBlocked" OR ` + catalogKeywordBlockedSQL + `)),
		(SELECT COUNT(*)::integer FROM "CatalogDanmakuBinding" b WHERE b."PoolId"=p."Id")
		FROM "CatalogDanmakuPool" p` + where + ` ORDER BY p."EpisodeId" LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := p.db.Query(ctx, query, args...)
	if err != nil {
		return domain.Page[domain.CatalogPool]{}, err
	}
	defer rows.Close()
	list := make([]domain.CatalogPool, 0)
	for rows.Next() {
		var item domain.CatalogPool
		if err := rows.Scan(&item.ID, &item.EpisodeID, &item.LastAttemptTime, &item.LastSyncTime, &item.CreateTime, &item.UpdateTime, &item.DanmakuCount, &item.BlockedCount, &item.BindingCount); err != nil {
			return domain.Page[domain.CatalogPool]{}, err
		}
		list = append(list, item)
	}
	return domain.Page[domain.CatalogPool]{Total: total, List: list}, rows.Err()
}

func (p *CatalogStore) CatalogDanmaku(ctx context.Context, filter CatalogDanmakuFilter) (domain.Page[domain.CatalogDanmaku], error) {
	filter.Page, filter.Size = normalizePage(filter.Page, filter.Size)
	blocked := `(d."IsBlocked" OR ` + catalogKeywordBlockedSQL + `)`
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
	if err := p.db.QueryRow(ctx, `SELECT COUNT(*) FROM "CatalogDanmaku" d`+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.CatalogDanmaku]{}, err
	}
	args = append(args, filter.Size, filter.Size*(filter.Page-1))
	query := `SELECT d."Id",d."PoolId",d."Data",` + blocked + `,d."IsBlocked",d."CreateTime",d."UpdateTime" FROM "CatalogDanmaku" d` + where + ` ORDER BY d."TimeMillis" DESC,d."Id" DESC LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := p.db.Query(ctx, query, args...)
	if err != nil {
		return domain.Page[domain.CatalogDanmaku]{}, err
	}
	defer rows.Close()
	list := make([]domain.CatalogDanmaku, 0)
	for rows.Next() {
		var item domain.CatalogDanmaku
		var raw []byte
		if err := rows.Scan(&item.ID, &item.PoolID, &raw, &item.IsBlocked, &item.ManuallyBlocked, &item.CreateTime, &item.UpdateTime); err != nil {
			return domain.Page[domain.CatalogDanmaku]{}, err
		}
		if err := json.Unmarshal(raw, &item.Data); err != nil {
			return domain.Page[domain.CatalogDanmaku]{}, err
		}
		list = append(list, item)
	}
	return domain.Page[domain.CatalogDanmaku]{Total: total, List: list}, rows.Err()
}

func (p *CatalogStore) SetCatalogDanmakuBlocked(ctx context.Context, id int64, blocked bool) (bool, error) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	tag, err := p.db.Exec(ctx, `UPDATE "CatalogDanmaku" SET "IsBlocked"=$2,"UpdateTime"=$3 WHERE "Id"=$1`, id, blocked, now)
	ok := tag.RowsAffected() > 0
	if err == nil && ok {
		p.invalidateDanmakuCache(ctx, p.source)
	}
	return ok, err
}

func (p *CatalogStore) CatalogKeywords(ctx context.Context) ([]domain.CatalogKeyword, error) {
	rows, err := p.db.Query(ctx, `SELECT k."Id",k."PoolId",COALESCE(p."EpisodeId",''),k."Keyword",k."CreateTime" FROM "CatalogDanmakuKeyword" k LEFT JOIN "CatalogDanmakuPool" p ON p."Id"=k."PoolId" ORDER BY k."PoolId" NULLS FIRST,k."Keyword"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.CatalogKeyword, 0)
	for rows.Next() {
		var item domain.CatalogKeyword
		if err := rows.Scan(&item.ID, &item.PoolID, &item.PoolEpisodeID, &item.Keyword, &item.CreateTime); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (p *CatalogStore) CreateCatalogKeyword(ctx context.Context, poolID *int, keyword string) (*domain.CatalogKeyword, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, fmt.Errorf("keyword cannot be empty")
	}
	hash := sha256.Sum256([]byte(strings.ToLower(keyword)))
	now := time.Now().UTC().Truncate(time.Millisecond)
	var poolValue any
	if poolID != nil {
		if *poolID < 1 {
			return nil, fmt.Errorf("invalid catalog pool")
		}
		poolValue = *poolID
	}
	var item domain.CatalogKeyword
	err := p.db.QueryRow(ctx, `INSERT INTO "CatalogDanmakuKeyword" ("PoolId","Keyword","KeywordHash","CreateTime") VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING RETURNING "Id","PoolId","Keyword","CreateTime"`, poolValue, keyword, hex.EncodeToString(hash[:]), now).Scan(&item.ID, &item.PoolID, &item.Keyword, &item.CreateTime)
	if errors.Is(err, pgx.ErrNoRows) {
		err = p.db.QueryRow(ctx, `SELECT "Id","PoolId","Keyword","CreateTime" FROM "CatalogDanmakuKeyword" WHERE "PoolId" IS NOT DISTINCT FROM $1::integer AND "KeywordHash"=$2`, poolValue, hex.EncodeToString(hash[:])).Scan(&item.ID, &item.PoolID, &item.Keyword, &item.CreateTime)
	}
	if err != nil {
		return nil, err
	}
	if item.PoolID != nil {
		pool, err := p.CatalogPool(ctx, *item.PoolID)
		if err != nil {
			return nil, err
		}
		item.PoolEpisodeID = pool.EpisodeID
	}
	p.invalidateDanmakuCache(ctx, p.source)
	return &item, nil
}

func (p *CatalogStore) DeleteCatalogKeyword(ctx context.Context, id int) (bool, error) {
	tag, err := p.db.Exec(ctx, `DELETE FROM "CatalogDanmakuKeyword" WHERE "Id"=$1`, id)
	ok := tag.RowsAffected() > 0
	if err == nil && ok {
		p.invalidateDanmakuCache(ctx, p.source)
	}
	return ok, err
}

const catalogBindingSelect = `SELECT b."Id",b."Vid",b."PoolId",p."EpisodeId",b."Offset",b."CreateTime",b."UpdateTime" FROM "CatalogDanmakuBinding" b JOIN "CatalogDanmakuPool" p ON p."Id"=b."PoolId"`

func (p *CatalogStore) CatalogBindingsByVID(ctx context.Context, vid string) ([]domain.CatalogBinding, error) {
	rows, err := p.db.Query(ctx, catalogBindingSelect+` WHERE b."Vid"=$1 ORDER BY b."Id"`, vid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCatalogBindings(rows)
}

func (p *CatalogStore) VideoCatalogBindings(ctx context.Context, videoID int) ([]domain.CatalogBinding, error) {
	rows, err := p.db.Query(ctx, catalogBindingSelect+` JOIN "Video" v ON v."Vid"=b."Vid" WHERE v."Id"=$1 ORDER BY b."Id"`, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCatalogBindings(rows)
}

func (p *CatalogStore) UpsertVideoCatalogBinding(ctx context.Context, videoID, poolID int, offset float64) (*domain.CatalogBinding, error) {
	var vid string
	var deleted bool
	if err := p.db.QueryRow(ctx, `SELECT "Vid","IsDelete" FROM "Video" WHERE "Id"=$1`, videoID).Scan(&vid, &deleted); err != nil {
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
	if err := p.db.QueryRow(ctx, `INSERT INTO "CatalogDanmakuBinding" ("Vid","PoolId","Offset","CreateTime","UpdateTime") VALUES ($1,$2,$3,$4,$4) ON CONFLICT ("Vid","PoolId") DO UPDATE SET "Offset"=EXCLUDED."Offset","UpdateTime"=EXCLUDED."UpdateTime" RETURNING "Id"`, vid, poolID, offset, now).Scan(&id); err != nil {
		return nil, err
	}
	return scanCatalogBinding(p.db.QueryRow(ctx, catalogBindingSelect+` WHERE b."Id"=$1`, id))
}

func (p *CatalogStore) DeleteVideoCatalogBinding(ctx context.Context, videoID, bindingID int) (bool, error) {
	tag, err := p.db.Exec(ctx, `DELETE FROM "CatalogDanmakuBinding" b USING "Video" v WHERE b."Id"=$1 AND v."Id"=$2 AND b."Vid"=v."Vid"`, bindingID, videoID)
	return tag.RowsAffected() > 0, err
}

func scanCatalogPool(row pgx.Row) (*domain.CatalogPool, error) {
	var item domain.CatalogPool
	err := row.Scan(&item.ID, &item.EpisodeID, &item.LastAttemptTime, &item.LastSyncTime, &item.CreateTime, &item.UpdateTime)
	return &item, err
}

func scanCatalogBinding(row pgx.Row) (*domain.CatalogBinding, error) {
	var item domain.CatalogBinding
	err := row.Scan(&item.ID, &item.Vid, &item.PoolID, &item.PoolEpisodeID, &item.Offset, &item.CreateTime, &item.UpdateTime)
	return &item, err
}

func scanCatalogBindings(rows pgx.Rows) ([]domain.CatalogBinding, error) {
	result := make([]domain.CatalogBinding, 0)
	for rows.Next() {
		item, err := scanCatalogBinding(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	return result, rows.Err()
}

func catalogPoolAdvisoryKey(vid string) int64 {
	hash := sha256.New()
	var size [8]byte
	for _, value := range []string{"danmaku:catalog-pool:v1", vid} {
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write([]byte(value))
	}
	return int64(binary.BigEndian.Uint64(hash.Sum(nil)[:8]))
}
