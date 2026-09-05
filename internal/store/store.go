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
	ErrUserDisabled        = errors.New("user is disabled")
	ErrUserNameConflict    = errors.New("user name already exists")
	ErrCASProfileReadOnly  = errors.New("CAS profile fields are read-only")
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

type UserFilter struct {
	Page  int
	Size  int
	Query string
	Role  string
}

type UserCreate struct {
	Name        string
	Password    string
	Role        int
	Email       string
	PhoneNumber string
}

type UserUpdate struct {
	Name        string
	Password    string
	Role        int
	Email       string
	PhoneNumber string
}

type BilibiliPoolFilter struct {
	Page  int
	Size  int
	Query string
}

type BilibiliDanmakuFilter struct {
	Page    int
	Size    int
	PoolID  int
	Query   string
	Blocked *bool
}

type BilibiliBindingFilter struct {
	Page  int
	Size  int
	Query string
}

type Repository interface {
	Initialize(context.Context) error
	Close()
	QueryByVid(context.Context, string) ([]domain.DanmakuData, error)
	Insert(context.Context, string, domain.DanmakuData, net.IP, domain.Referer) (bool, error)
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
	Users(context.Context, UserFilter) (domain.Page[domain.User], error)
	CreateUser(context.Context, UserCreate) (*domain.User, error)
	UpdateUser(context.Context, int, UserUpdate) (*domain.User, error)
	SetUserEnabled(context.Context, int, bool) (bool, error)
	DeleteUser(context.Context, int) (bool, error)
	UpsertCASUser(context.Context, domain.CASProfile, int, bool) (*domain.User, bool, error)
	BilibiliPool(context.Context, int) (*domain.BilibiliPool, error)
	BilibiliPoolByKey(context.Context, string, int) (*domain.BilibiliPool, error)
	EnsureBilibiliPool(context.Context, string, int, int64) (*domain.BilibiliPool, error)
	ClaimBilibiliPoolSync(context.Context, int, time.Duration, bool) (bool, error)
	MergeBilibiliDanmaku(context.Context, int, []domain.DanmakuData) (int, error)
	BilibiliPoolData(context.Context, int) ([]domain.DanmakuData, error)
	BilibiliPools(context.Context, BilibiliPoolFilter) (domain.Page[domain.BilibiliPool], error)
	BilibiliDanmaku(context.Context, BilibiliDanmakuFilter) (domain.Page[domain.BilibiliDanmaku], error)
	SetBilibiliDanmakuBlocked(context.Context, int64, bool) (bool, error)
	BilibiliKeywords(context.Context) ([]domain.BilibiliKeyword, error)
	CreateBilibiliKeyword(context.Context, *int, string) (*domain.BilibiliKeyword, error)
	DeleteBilibiliKeyword(context.Context, int) (bool, error)
	BilibiliBindings(context.Context, BilibiliBindingFilter) (domain.Page[domain.BilibiliBinding], error)
	BilibiliBindingsByVID(context.Context, string) ([]domain.BilibiliBinding, error)
	UpsertBilibiliBinding(context.Context, string, int, float64) (*domain.BilibiliBinding, error)
	DeleteBilibiliBinding(context.Context, int) (bool, error)
	Cache(context.Context, string, time.Duration, func(context.Context) ([]byte, error)) ([]byte, error)
}
