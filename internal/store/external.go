package store

import (
	"context"
	"crypto/sha256"
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

const externalPoolSelect = `SELECT p."Id"::text,p."Name",p."SourceFormat",p."CreateTime",p."UpdateTime",
	(SELECT COUNT(*)::integer FROM "ExternalDanmaku" d WHERE d."PoolId"=p."Id"),
	(SELECT COUNT(*)::integer FROM "ExternalDanmakuBinding" b WHERE b."PoolId"=p."Id"),
	(SELECT COUNT(*)::integer FROM "ExternalDanmaku" d WHERE d."PoolId"=p."Id" AND ` + externalKeywordBlockedSQL + `)
	FROM "ExternalDanmakuPool" p`

func (p *Postgres) ExternalPool(ctx context.Context, id string) (*domain.ExternalPool, error) {
	if !validUUID(id) {
		return nil, nil
	}
	pool, err := scanExternalPool(p.pool.QueryRow(ctx, externalPoolSelect+` WHERE p."Id"=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	pool.Bindings, err = p.externalPoolBindings(ctx, id)
	return pool, err
}

func (p *Postgres) ExternalPools(ctx context.Context, filter ExternalPoolFilter) (domain.Page[domain.ExternalPool], error) {
	filter.Page, filter.Size = normalizePage(filter.Page, filter.Size)
	args := make([]any, 0, 3)
	where := ""
	if query := strings.TrimSpace(filter.Query); query != "" {
		args = append(args, query)
		where = ` WHERE p."Id"::text ILIKE '%' || $1 || '%' OR p."Name" ILIKE '%' || $1 || '%' OR p."SourceFormat" ILIKE '%' || $1 || '%'`
	}
	var total int
	if err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM "ExternalDanmakuPool" p`+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.ExternalPool]{}, err
	}
	args = append(args, filter.Size, filter.Size*(filter.Page-1))
	rows, err := p.pool.Query(ctx, externalPoolSelect+where+fmt.Sprintf(` ORDER BY p."UpdateTime" DESC,p."Id" LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return domain.Page[domain.ExternalPool]{}, err
	}
	defer rows.Close()
	list := make([]domain.ExternalPool, 0, filter.Size)
	for rows.Next() {
		item, err := scanExternalPool(rows)
		if err != nil {
			return domain.Page[domain.ExternalPool]{}, err
		}
		list = append(list, *item)
	}
	return domain.Page[domain.ExternalPool]{Total: total, List: list}, rows.Err()
}

func (p *Postgres) CreateExternalPool(ctx context.Context, name, sourceFormat string, data []domain.DanmakuData) (*domain.ExternalPool, error) {
	id, err := randomUUID()
	if err != nil {
		return nil, err
	}
	return p.writeExternalPool(ctx, id, name, sourceFormat, data, true)
}

func (p *Postgres) ReplaceExternalPool(ctx context.Context, id, name, sourceFormat string, data []domain.DanmakuData) (*domain.ExternalPool, error) {
	if !validUUID(id) {
		return nil, nil
	}
	return p.writeExternalPool(ctx, id, name, sourceFormat, data, false)
}

func (p *Postgres) writeExternalPool(ctx context.Context, id, name, sourceFormat string, data []domain.DanmakuData, create bool) (*domain.ExternalPool, error) {
	name, sourceFormat = strings.TrimSpace(name), strings.TrimSpace(sourceFormat)
	if name == "" || len([]rune(name)) > 200 || sourceFormat == "" || len([]rune(sourceFormat)) > 64 {
		return nil, fmt.Errorf("invalid external pool fields")
	}
	if len(data) > 1_000_000 {
		return nil, fmt.Errorf("external pool contains too many danmaku entries")
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := time.Now().UTC().Truncate(time.Millisecond)
	if create {
		if _, err := tx.Exec(ctx, `INSERT INTO "ExternalDanmakuPool" ("Id","Name","SourceFormat","CreateTime","UpdateTime") VALUES ($1,$2,$3,$4,$4)`, id, name, sourceFormat, now); err != nil {
			return nil, err
		}
	} else {
		tag, err := tx.Exec(ctx, `UPDATE "ExternalDanmakuPool" SET "Name"=$2,"SourceFormat"=$3,"UpdateTime"=$4 WHERE "Id"=$1`, id, name, sourceFormat, now)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			return nil, nil
		}
		if _, err := tx.Exec(ctx, `DELETE FROM "ExternalDanmaku" WHERE "PoolId"=$1`, id); err != nil {
			return nil, err
		}
	}
	if err := insertExternalDanmaku(ctx, tx, id, data, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	p.invalidateDanmakuCache(ctx, "external")
	return p.ExternalPool(ctx, id)
}

func insertExternalDanmaku(ctx context.Context, tx pgx.Tx, poolID string, data []domain.DanmakuData, now time.Time) error {
	const chunkSize = 2000
	for start := 0; start < len(data); start += chunkSize {
		end := min(start+chunkSize, len(data))
		batch := &pgx.Batch{}
		for _, item := range data[start:end] {
			if math.IsNaN(float64(item.Time)) || math.IsInf(float64(item.Time), 0) || item.Time < 0 || item.Text == nil {
				continue
			}
			content := *item.Text
			if strings.TrimSpace(content) == "" {
				continue
			}
			raw, err := domain.MarshalDBData(item)
			if err != nil {
				return err
			}
			hash := sha256.Sum256([]byte(content))
			batch.Queue(`INSERT INTO "ExternalDanmaku" ("PoolId","TimeMillis","Content","ContentHash","Data","CreateTime","UpdateTime") VALUES ($1,$2,$3,$4,$5,$6,$6) ON CONFLICT ("PoolId","TimeMillis","ContentHash") DO NOTHING`, poolID, int64(math.Round(float64(item.Time)*1000)), content, hex.EncodeToString(hash[:]), raw, now)
		}
		if batch.Len() == 0 {
			continue
		}
		results := tx.SendBatch(ctx, batch)
		for i := 0; i < batch.Len(); i++ {
			if _, err := results.Exec(); err != nil {
				_ = results.Close()
				return err
			}
		}
		if err := results.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (p *Postgres) ExternalPoolData(ctx context.Context, poolID string) ([]domain.DanmakuData, error) {
	if !validUUID(poolID) {
		return []domain.DanmakuData{}, nil
	}
	return p.cachedDanmaku(ctx, "external", poolID, func(ctx context.Context) ([]domain.DanmakuData, error) {
		rows, err := p.pool.Query(ctx, `SELECT d."Data" FROM "ExternalDanmaku" d WHERE d."PoolId"=$1 AND NOT `+externalKeywordBlockedSQL+` ORDER BY d."TimeMillis",d."Id"`, poolID)
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
	})
}

func (p *Postgres) ExternalDanmaku(ctx context.Context, filter ExternalDanmakuFilter) (domain.Page[domain.ExternalDanmaku], error) {
	filter.Page, filter.Size = normalizePage(filter.Page, filter.Size)
	if !validUUID(filter.PoolID) {
		return domain.Page[domain.ExternalDanmaku]{List: []domain.ExternalDanmaku{}}, nil
	}
	args := []any{filter.PoolID}
	where := ` WHERE "PoolId"=$1`
	if query := strings.TrimSpace(filter.Query); query != "" {
		args = append(args, query)
		where += fmt.Sprintf(` AND "Content" ILIKE '%%' || $%d || '%%'`, len(args))
	}
	var total int
	if err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM "ExternalDanmaku"`+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.ExternalDanmaku]{}, err
	}
	args = append(args, filter.Size, filter.Size*(filter.Page-1))
	rows, err := p.pool.Query(ctx, `SELECT "Id","PoolId"::text,"Data","CreateTime","UpdateTime",`+externalKeywordBlockedSQL+` FROM "ExternalDanmaku" d`+where+fmt.Sprintf(` ORDER BY "TimeMillis","Id" LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return domain.Page[domain.ExternalDanmaku]{}, err
	}
	defer rows.Close()
	list := make([]domain.ExternalDanmaku, 0, filter.Size)
	for rows.Next() {
		var item domain.ExternalDanmaku
		var raw []byte
		if err := rows.Scan(&item.ID, &item.PoolID, &raw, &item.CreateTime, &item.UpdateTime, &item.KeywordBlocked); err != nil {
			return domain.Page[domain.ExternalDanmaku]{}, err
		}
		if err := json.Unmarshal(raw, &item.Data); err != nil {
			return domain.Page[domain.ExternalDanmaku]{}, err
		}
		list = append(list, item)
	}
	return domain.Page[domain.ExternalDanmaku]{Total: total, List: list}, rows.Err()
}

const externalBindingSelect = `SELECT b."Id",b."Vid",b."PoolId"::text,p."Name",b."Offset",b."CreateTime",b."UpdateTime" FROM "ExternalDanmakuBinding" b JOIN "ExternalDanmakuPool" p ON p."Id"=b."PoolId"`

func (p *Postgres) ExternalBindingsByVID(ctx context.Context, vid string) ([]domain.ExternalBinding, error) {
	rows, err := p.pool.Query(ctx, externalBindingSelect+` WHERE b."Vid"=$1 ORDER BY b."Id"`, vid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanExternalBindings(rows)
}

func (p *Postgres) VideoExternalBindings(ctx context.Context, videoID int) ([]domain.ExternalBinding, error) {
	rows, err := p.pool.Query(ctx, externalBindingSelect+` JOIN "Video" v ON v."Vid"=b."Vid" WHERE v."Id"=$1 ORDER BY b."Id"`, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanExternalBindings(rows)
}

func (p *Postgres) externalPoolBindings(ctx context.Context, poolID string) ([]domain.ExternalBinding, error) {
	rows, err := p.pool.Query(ctx, externalBindingSelect+` WHERE b."PoolId"=$1 ORDER BY b."Id"`, poolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanExternalBindings(rows)
}

func (p *Postgres) UpsertVideoExternalBinding(ctx context.Context, videoID int, poolID string, offset float64) (*domain.ExternalBinding, error) {
	if !validUUID(poolID) {
		return nil, fmt.Errorf("invalid external pool")
	}
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
	if err := p.pool.QueryRow(ctx, `INSERT INTO "ExternalDanmakuBinding" ("Vid","PoolId","Offset","CreateTime","UpdateTime") VALUES ($1,$2,$3,$4,$4) ON CONFLICT ("Vid","PoolId") DO UPDATE SET "Offset"=EXCLUDED."Offset","UpdateTime"=EXCLUDED."UpdateTime" RETURNING "Id"`, vid, poolID, offset, now).Scan(&id); err != nil {
		return nil, err
	}
	return scanExternalBinding(p.pool.QueryRow(ctx, externalBindingSelect+` WHERE b."Id"=$1`, id))
}

func (p *Postgres) DeleteVideoExternalBinding(ctx context.Context, videoID, bindingID int) (bool, error) {
	tag, err := p.pool.Exec(ctx, `DELETE FROM "ExternalDanmakuBinding" b USING "Video" v WHERE b."Id"=$1 AND v."Id"=$2 AND b."Vid"=v."Vid"`, bindingID, videoID)
	return tag.RowsAffected() > 0, err
}

func scanExternalPool(row pgx.Row) (*domain.ExternalPool, error) {
	var item domain.ExternalPool
	err := row.Scan(&item.ID, &item.Name, &item.SourceFormat, &item.CreateTime, &item.UpdateTime, &item.DanmakuCount, &item.BindingCount, &item.BlockedCount)
	return &item, err
}

func scanExternalBinding(row pgx.Row) (*domain.ExternalBinding, error) {
	var item domain.ExternalBinding
	err := row.Scan(&item.ID, &item.Vid, &item.PoolID, &item.PoolName, &item.Offset, &item.CreateTime, &item.UpdateTime)
	return &item, err
}

func scanExternalBindings(rows pgx.Rows) ([]domain.ExternalBinding, error) {
	result := make([]domain.ExternalBinding, 0)
	for rows.Next() {
		item, err := scanExternalBinding(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	return result, rows.Err()
}
