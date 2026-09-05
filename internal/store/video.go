package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"github.com/jackc/pgx/v5"
)

const managedVideoSelect = `SELECT v."Id",v."Vid",COALESCE(v."Name",''),v."IsDelete",TRUE,
	(SELECT COUNT(*)::integer FROM "Danmaku" d WHERE d."VideoId"=v."Id" AND NOT d."IsDelete"),
	(SELECT COUNT(*)::integer FROM "BilibiliDanmakuBinding" b WHERE b."Vid"=v."Vid"),
	v."CreateTime",v."UpdateTime" FROM "Video" v`

func (p *Postgres) EnsureVideo(ctx context.Context, vid string) (*domain.Video, error) {
	vid = strings.TrimSpace(vid)
	if vid == "" || len([]rune(vid)) > 36 {
		return nil, fmt.Errorf("invalid video ID")
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	if _, err := p.pool.Exec(ctx, `INSERT INTO "Video" ("Vid","Referer","Name","IsDelete","CreateTime","UpdateTime") VALUES ($1,NULL,NULL,FALSE,$2,$2) ON CONFLICT ("Vid") DO NOTHING`, vid, now); err != nil {
		return nil, err
	}
	return scanManagedVideo(p.pool.QueryRow(ctx, managedVideoSelect+` WHERE v."Vid"=$1`, vid))
}

func (p *Postgres) Videos(ctx context.Context, filter VideoFilter) (domain.Page[domain.Video], error) {
	filter.Page, filter.Size = normalizePage(filter.Page, filter.Size)
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 4)
	if query := strings.TrimSpace(filter.Query); query != "" {
		args = append(args, query)
		clauses = append(clauses, fmt.Sprintf(`(v."Vid" ILIKE '%%' || $%d || '%%' OR COALESCE(v."Name",'') ILIKE '%%' || $%d || '%%')`, len(args), len(args)))
	}
	if filter.IsDeleted != nil {
		args = append(args, *filter.IsDeleted)
		clauses = append(clauses, fmt.Sprintf(`v."IsDelete"=$%d`, len(args)))
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	var total int
	if err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM "Video" v`+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.Video]{}, err
	}
	args = append(args, filter.Size, filter.Size*(filter.Page-1))
	query := managedVideoSelect + where + fmt.Sprintf(` ORDER BY v."UpdateTime" DESC,v."Id" DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return domain.Page[domain.Video]{}, err
	}
	defer rows.Close()
	list := make([]domain.Video, 0, filter.Size)
	for rows.Next() {
		item, err := scanManagedVideo(rows)
		if err != nil {
			return domain.Page[domain.Video]{}, err
		}
		list = append(list, *item)
	}
	return domain.Page[domain.Video]{Total: total, List: list}, rows.Err()
}

func (p *Postgres) Video(ctx context.Context, id int) (*domain.Video, error) {
	item, err := scanManagedVideo(p.pool.QueryRow(ctx, managedVideoSelect+` WHERE v."Id"=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item.BilibiliBindings, err = p.VideoBilibiliBindings(ctx, id)
	return item, err
}

func (p *Postgres) CreateVideo(ctx context.Context, vid, name string) (*domain.Video, error) {
	vid, name = strings.TrimSpace(vid), strings.TrimSpace(name)
	if vid == "" || len([]rune(vid)) > 36 || len([]rune(name)) > 200 {
		return nil, fmt.Errorf("invalid video fields")
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	var id int
	err := p.pool.QueryRow(ctx, `INSERT INTO "Video" ("Vid","Referer","Name","IsDelete","CreateTime","UpdateTime") VALUES ($1,NULL,NULLIF($2,''),FALSE,$3,$3) ON CONFLICT ("Vid") DO NOTHING RETURNING "Id"`, vid, name, now).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrVideoExists
	}
	if err != nil {
		return nil, err
	}
	return p.Video(ctx, id)
}

func (p *Postgres) UpdateVideo(ctx context.Context, id int, name string) (*domain.Video, error) {
	name = strings.TrimSpace(name)
	if len([]rune(name)) > 200 {
		return nil, fmt.Errorf("invalid video name")
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	tag, err := p.pool.Exec(ctx, `UPDATE "Video" SET "Name"=NULLIF($2,''),"UpdateTime"=$3 WHERE "Id"=$1`, id, name, now)
	if err != nil || tag.RowsAffected() == 0 {
		return nil, err
	}
	return p.Video(ctx, id)
}

func (p *Postgres) SetVideoDeleted(ctx context.Context, id int, deleted bool) (bool, error) {
	tag, err := p.pool.Exec(ctx, `UPDATE "Video" SET "IsDelete"=$2,"UpdateTime"=$3 WHERE "Id"=$1`, id, deleted, time.Now().UTC().Truncate(time.Millisecond))
	return tag.RowsAffected() > 0, err
}

func (p *Postgres) findOrInsertVideo(ctx context.Context, tx pgx.Tx, vid string) (int, bool, error) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	if _, err := tx.Exec(ctx, `INSERT INTO "Video" ("Vid","Referer","Name","IsDelete","CreateTime","UpdateTime") VALUES ($1,NULL,NULL,FALSE,$2,$2) ON CONFLICT ("Vid") DO NOTHING`, vid, now); err != nil {
		return 0, false, err
	}
	var id int
	var deleted bool
	err := tx.QueryRow(ctx, `SELECT "Id","IsDelete" FROM "Video" WHERE "Vid"=$1 FOR UPDATE`, vid).Scan(&id, &deleted)
	return id, deleted, err
}

func scanManagedVideo(row pgx.Row) (*domain.Video, error) {
	var item domain.Video
	err := row.Scan(&item.ID, &item.Vid, &item.Name, &item.IsDeleted, &item.DefaultPool, &item.DanmakuCount, &item.BilibiliPoolCount, &item.CreateTime, &item.UpdateTime)
	return &item, err
}
