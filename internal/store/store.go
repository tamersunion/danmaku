package store

import (
	"context"
	"errors"
	"net"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
)

var (
	ErrCASUserNotFound     = errors.New("CAS user does not exist and automatic creation is disabled")
	ErrCASIdentityConflict = errors.New("CAS identity conflicts with an existing user")
)

type SearchFilter struct {
	Page       int
	Size       int
	Vid        string
	Author     string
	AuthorID   *int
	Start      *time.Time
	End        *time.Time
	Mode       *int
	IP         net.IP
	Key        string
	Descending bool
}

type Repository interface {
	Initialize(context.Context) error
	Close()
	QueryByVid(context.Context, string) ([]domain.DanmakuData, error)
	Insert(context.Context, string, domain.DanmakuData, net.IP, domain.Referer) error
	List(context.Context, string, int, int, bool) (domain.Page[domain.Danmaku], error)
	Search(context.Context, SearchFilter) (domain.Page[domain.Danmaku], error)
	Vids(context.Context) ([]string, error)
	Get(context.Context, string) (*domain.Danmaku, error)
	Edit(context.Context, string, domain.DanmakuData, bool) (*domain.Danmaku, error)
	Delete(context.Context, string) (bool, error)
	VerifyPassword(context.Context, string, string) (bool, int, int, error)
	ChangePassword(context.Context, int, string, string) (bool, error)
	ChangeUserInfo(context.Context, int, string, *string, *string) (bool, error)
	User(context.Context, int) (*domain.User, error)
	UpsertCASUser(context.Context, domain.CASProfile, int, bool) (*domain.User, bool, error)
	Cache(context.Context, string, time.Duration, func(context.Context) ([]byte, error)) ([]byte, error)
}
