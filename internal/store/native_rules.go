package store

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"github.com/jackc/pgx/v5"
)

var ErrSubmissionDenied = errors.New("submission denied")
var ErrInvalidNativeRule = errors.New("invalid native rule")
var ErrNativeRuleExists = errors.New("native rule already exists")

// Two-key advisory locks use a separate PostgreSQL key space from the
// duplicate guard. Shared submission locks allow concurrent sends; rule writes
// are exclusive so historical replacement cannot miss an in-flight old author.
const nativeRuleReadLock = `SELECT pg_advisory_xact_lock_shared(1684106861, 1)`
const nativeRuleWriteLock = `SELECT pg_advisory_xact_lock(1684106861, 1)`

type NativeRule struct {
	ID          int       `json:"id"`
	Kind        string    `json:"kind"`
	Value       string    `json:"value"`
	Replacement string    `json:"replacement"`
	CreateTime  time.Time `json:"createTime"`
}

type NativeRuleInput struct {
	Value        string `json:"value"`
	Replacement  string `json:"replacement"`
	ScanExisting bool   `json:"scanExisting"`
}

type NativeRuleResult struct {
	Rule     NativeRule `json:"rule"`
	Replaced int64      `json:"replaced"`
}

type NativeRuleRepository interface {
	NativeRules(context.Context, string) ([]NativeRule, error)
	CreateNativeRule(context.Context, string, NativeRuleInput) (*NativeRuleResult, error)
	DeleteNativeRule(context.Context, string, int) (bool, error)
}

func ValidNativeRuleKind(kind string) bool {
	return kind == "keywords" || kind == "authors" || kind == "ips"
}

func normalizeNativeRule(kind string, input NativeRuleInput) (NativeRuleInput, error) {
	if !ValidNativeRuleKind(kind) {
		return input, ErrInvalidNativeRule
	}
	if kind != "authors" {
		input.Value = strings.TrimSpace(input.Value)
	}
	if strings.TrimSpace(input.Value) == "" || len([]rune(input.Value)) > 200 || strings.ContainsRune(input.Value, 0) {
		return input, ErrInvalidNativeRule
	}
	if kind == "authors" {
		if strings.TrimSpace(input.Replacement) == "" || len([]rune(input.Replacement)) > 200 || strings.ContainsRune(input.Replacement, 0) || input.Value == input.Replacement {
			return input, ErrInvalidNativeRule
		}
	} else if input.Replacement != "" || input.ScanExisting {
		return input, ErrInvalidNativeRule
	}
	if kind == "keywords" {
		input.Value = strings.ToLower(input.Value)
	}
	if kind == "ips" {
		prefix, err := netip.ParsePrefix(input.Value)
		if err != nil {
			addr, e := netip.ParseAddr(input.Value)
			if e != nil || addr.Zone() != "" {
				return input, ErrInvalidNativeRule
			}
			addr = addr.Unmap()
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		if prefix.Addr().Is4In6() {
			if prefix.Bits() < 96 {
				return input, ErrInvalidNativeRule
			}
			prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
		}
		input.Value = prefix.Masked().String()
	}
	return input, nil
}

func (p *Postgres) NativeRules(ctx context.Context, kind string) ([]NativeRule, error) {
	if !ValidNativeRuleKind(kind) {
		return nil, ErrInvalidNativeRule
	}
	rows, err := p.pool.Query(ctx, `SELECT "Id","Kind","Value","Replacement","CreateTime" FROM "NativeDanmakuRule" WHERE "Kind"=$1 ORDER BY "Id" DESC`, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := []NativeRule{}
	for rows.Next() {
		var rule NativeRule
		if err := rows.Scan(&rule.ID, &rule.Kind, &rule.Value, &rule.Replacement, &rule.CreateTime); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

const nativeAuthorSQL = `COALESCE(NULLIF(COALESCE("Data"->>'Author',"Data"->>'author'),''),"Data"->>'AuthorId',"Data"->>'authorId','0')`

func (p *Postgres) CreateNativeRule(ctx context.Context, kind string, input NativeRuleInput) (*NativeRuleResult, error) {
	input, err := normalizeNativeRule(kind, input)
	if err != nil {
		return nil, err
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, nativeRuleWriteLock); err != nil {
		return nil, err
	}
	result := &NativeRuleResult{}
	err = tx.QueryRow(ctx, `INSERT INTO "NativeDanmakuRule" ("Kind","Value","Replacement","CreateTime") VALUES ($1,$2,$3,$4) ON CONFLICT ("Kind","Value") DO NOTHING RETURNING "Id","Kind","Value","Replacement","CreateTime"`, kind, input.Value, input.Replacement, time.Now().UTC()).Scan(&result.Rule.ID, &result.Rule.Kind, &result.Rule.Value, &result.Rule.Replacement, &result.Rule.CreateTime)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNativeRuleExists
	}
	if err != nil {
		return nil, err
	}
	if input.ScanExisting {
		tag, err := tx.Exec(ctx, `UPDATE "Danmaku" SET "Data"=("Data"-'author') || jsonb_build_object('Author',$2::text),"UpdateTime"=$3 WHERE `+nativeAuthorSQL+`=$1`, input.Value, input.Replacement, time.Now().UTC().Truncate(time.Second))
		if err != nil {
			return nil, err
		}
		result.Replaced = tag.RowsAffected()
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if input.ScanExisting {
		p.invalidateDanmakuCache(ctx, "native")
	}
	return result, nil
}

func (p *Postgres) DeleteNativeRule(ctx context.Context, kind string, id int) (bool, error) {
	if !ValidNativeRuleKind(kind) || id < 1 {
		return false, ErrInvalidNativeRule
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, nativeRuleWriteLock); err != nil {
		return false, err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM "NativeDanmakuRule" WHERE "Kind"=$1 AND "Id"=$2`, kind, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, tx.Commit(ctx)
}

func applyNativeRules(data domain.DanmakuData, ip net.IP, rules []NativeRule) (domain.DanmakuData, bool, error) {
	author := data.Author
	if author == "" {
		author = strconv.Itoa(data.AuthorID)
	}
	text := ""
	if data.Text != nil {
		text = strings.ToLower(*data.Text)
	}
	addr, _ := netip.AddrFromSlice(ip)
	addr = addr.Unmap()
	deleted := false
	for _, rule := range rules {
		switch rule.Kind {
		case "ips":
			prefix, err := netip.ParsePrefix(rule.Value)
			if err != nil {
				return data, false, fmt.Errorf("invalid stored IP rule: %w", err)
			}
			if prefix.Contains(addr) {
				return data, false, ErrSubmissionDenied
			}
		case "keywords":
			if strings.Contains(text, rule.Value) {
				deleted = true
			}
		case "authors":
			if author == rule.Value {
				data.Author = rule.Replacement
			}
		}
	}
	return data, deleted, nil
}

func submissionRules(ctx context.Context, tx pgx.Tx) ([]NativeRule, error) {
	if _, err := tx.Exec(ctx, nativeRuleReadLock); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT "Kind","Value","Replacement" FROM "NativeDanmakuRule" ORDER BY "Id"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := []NativeRule{}
	for rows.Next() {
		var r NativeRule
		if err := rows.Scan(&r.Kind, &r.Value, &r.Replacement); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}
