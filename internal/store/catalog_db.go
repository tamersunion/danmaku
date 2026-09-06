package store

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"regexp"
	"strings"
)

var catalogNames = map[string]string{"bahamut": "Bahamut", "tencent": "Tencent", "youku": "Youku"}
var catalogID = regexp.MustCompile(`^[A-Za-z0-9_=-]{1,128}$`)

func validCatalogID(id string) bool { return catalogID.MatchString(id) }

// SQL templates use Catalog table identifiers. Only the fixed provider allowlist
// can replace them; all user input remains a bound PostgreSQL parameter.
type catalogQueryer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Begin(context.Context) (pgx.Tx, error)
}
type catalogDB struct {
	catalogQueryer
	prefix string
}

func (d catalogDB) sql(q string) string {
	return strings.ReplaceAll(q, "CatalogDanmaku", d.prefix+"Danmaku")
}
func (d catalogDB) Exec(c context.Context, q string, a ...any) (pgconn.CommandTag, error) {
	return d.catalogQueryer.Exec(c, d.sql(q), a...)
}
func (d catalogDB) Query(c context.Context, q string, a ...any) (pgx.Rows, error) {
	return d.catalogQueryer.Query(c, d.sql(q), a...)
}
func (d catalogDB) QueryRow(c context.Context, q string, a ...any) pgx.Row {
	return d.catalogQueryer.QueryRow(c, d.sql(q), a...)
}
func (d catalogDB) Begin(c context.Context) (*catalogTx, error) {
	t, e := d.catalogQueryer.Begin(c)
	return &catalogTx{Tx: t, db: catalogDB{t, d.prefix}}, e
}

type catalogTx struct {
	pgx.Tx
	db catalogDB
}

func (t *catalogTx) Exec(c context.Context, q string, a ...any) (pgconn.CommandTag, error) {
	return t.db.Exec(c, q, a...)
}
func (t *catalogTx) QueryRow(c context.Context, q string, a ...any) pgx.Row {
	return t.db.QueryRow(c, q, a...)
}

type CatalogStore struct {
	*Postgres
	source string
	db     catalogDB
}

func (p *Postgres) Catalog(source string) CatalogRepository {
	prefix, ok := catalogNames[source]
	if !ok {
		return nil
	}
	return &CatalogStore{Postgres: p, source: source, db: catalogDB{p.pool, prefix}}
}
func catalogSchemaStatements() []string {
	var result []string
	// Reuse the proven single-episode pool schema, but keep provider storage,
	// filtering, IDs and sync windows completely separate.
	for _, source := range []string{"bahamut", "tencent", "youku"} {
		for _, statement := range schemaStatements() {
			if strings.Contains(statement, "AnimekoDanmaku") {
				result = append(result, strings.ReplaceAll(statement, "Animeko", catalogNames[source]))
			}
		}
	}
	return result
}
func catalogSizeSQL() string {
	var parts []string
	for _, source := range []string{"bahamut", "tencent", "youku"} {
		prefix := catalogNames[source]
		parts = append(parts, fmt.Sprintf(`SELECT '%s:' || b."PoolId"::text,COUNT(d."Id") FROM "%sDanmakuBinding" b LEFT JOIN "%sDanmaku" d ON d."PoolId"=b."PoolId" WHERE b."Vid"=$1 GROUP BY b."PoolId"`, source, prefix, prefix))
	}
	return strings.Join(parts, " UNION ALL ")
}
