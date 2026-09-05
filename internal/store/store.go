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
	ErrVideoDeleted        = errors.New("video is deleted")
	ErrVideoExists         = errors.New("video already exists")
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

type IqiyiPoolFilter = BilibiliPoolFilter
type IqiyiDanmakuFilter = BilibiliDanmakuFilter

type ExternalPoolFilter = BilibiliPoolFilter

type ExternalDanmakuFilter struct {
	Page   int
	Size   int
	PoolID string
	Query  string
}

type VideoFilter struct {
	Page      int
	Size      int
	Query     string
	IsDeleted *bool
}

type Repository interface {
	Initialize(context.Context) error
	Close()
	QueryByVid(context.Context, string) ([]domain.DanmakuData, error)
	Insert(context.Context, string, domain.DanmakuData, net.IP, domain.Referer) (bool, error)
	List(context.Context, string, int, int, bool) (domain.Page[domain.Danmaku], error)
	Search(context.Context, SearchFilter) (domain.Page[domain.Danmaku], error)
	Vids(context.Context) ([]string, error)
	EnsureVideo(context.Context, string) (*domain.Video, error)
	Videos(context.Context, VideoFilter) (domain.Page[domain.Video], error)
	Video(context.Context, int) (*domain.Video, error)
	CreateVideo(context.Context, string, string) (*domain.Video, error)
	UpdateVideo(context.Context, int, string) (*domain.Video, error)
	SetVideoDeleted(context.Context, int, bool) (bool, error)
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
	BilibiliBindingsByVID(context.Context, string) ([]domain.BilibiliBinding, error)
	VideoBilibiliBindings(context.Context, int) ([]domain.BilibiliBinding, error)
	UpsertVideoBilibiliBinding(context.Context, int, int, float64) (*domain.BilibiliBinding, error)
	DeleteVideoBilibiliBinding(context.Context, int, int) (bool, error)
	IqiyiPool(context.Context, int) (*domain.IqiyiPool, error)
	EnsureIqiyiPool(context.Context, string) (*domain.IqiyiPool, error)
	ClaimIqiyiPoolSync(context.Context, int, time.Duration, bool) (bool, error)
	MergeIqiyiDanmaku(context.Context, int, []domain.DanmakuData) (int, error)
	IqiyiPoolData(context.Context, int) ([]domain.DanmakuData, error)
	IqiyiPools(context.Context, IqiyiPoolFilter) (domain.Page[domain.IqiyiPool], error)
	IqiyiDanmaku(context.Context, IqiyiDanmakuFilter) (domain.Page[domain.IqiyiDanmaku], error)
	SetIqiyiDanmakuBlocked(context.Context, int64, bool) (bool, error)
	IqiyiKeywords(context.Context) ([]domain.IqiyiKeyword, error)
	CreateIqiyiKeyword(context.Context, *int, string) (*domain.IqiyiKeyword, error)
	DeleteIqiyiKeyword(context.Context, int) (bool, error)
	IqiyiBindingsByVID(context.Context, string) ([]domain.IqiyiBinding, error)
	VideoIqiyiBindings(context.Context, int) ([]domain.IqiyiBinding, error)
	UpsertVideoIqiyiBinding(context.Context, int, int, float64) (*domain.IqiyiBinding, error)
	DeleteVideoIqiyiBinding(context.Context, int, int) (bool, error)
	Cache(context.Context, string, time.Duration, func(context.Context) ([]byte, error)) ([]byte, error)
}

// ExternalRepository is kept separate so existing integrations implementing
// Repository remain source compatible while the manually imported source is
// additive.
type ExternalRepository interface {
	ExternalPool(context.Context, string) (*domain.ExternalPool, error)
	ExternalPools(context.Context, ExternalPoolFilter) (domain.Page[domain.ExternalPool], error)
	CreateExternalPool(context.Context, string, string, []domain.DanmakuData) (*domain.ExternalPool, error)
	ReplaceExternalPool(context.Context, string, string, string, []domain.DanmakuData) (*domain.ExternalPool, error)
	ExternalPoolData(context.Context, string) ([]domain.DanmakuData, error)
	ExternalDanmaku(context.Context, ExternalDanmakuFilter) (domain.Page[domain.ExternalDanmaku], error)
	ExternalBindingsByVID(context.Context, string) ([]domain.ExternalBinding, error)
	VideoExternalBindings(context.Context, int) ([]domain.ExternalBinding, error)
	UpsertVideoExternalBinding(context.Context, int, string, float64) (*domain.ExternalBinding, error)
	DeleteVideoExternalBinding(context.Context, int, int) (bool, error)
}
