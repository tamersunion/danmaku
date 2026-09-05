package domain

import (
	"encoding/json"
	"net"
	"time"
)

type DanmakuData struct {
	Time      float32 `json:"time"`
	Mode      int     `json:"mode"`
	Size      int     `json:"size"`
	Color     int     `json:"color"`
	Timestamp int64   `json:"timeStamp"`
	Pool      int     `json:"pool"`
	Author    string  `json:"author"`
	AuthorID  int     `json:"authorId"`
	Text      *string `json:"text"`
}

func NewDanmakuData() DanmakuData {
	empty := ""
	return DanmakuData{Size: 25, Timestamp: time.Now().Unix(), Author: "", Text: &empty}
}

type DanmakuInput struct {
	DanmakuData
	ID      string `json:"id"`
	IP      net.IP `json:"ip"`
	Referer string `json:"referer"`
}

type DPlayerInput struct {
	Time    float32 `json:"time"`
	Type    int     `json:"type"`
	Color   int     `json:"color"`
	Author  string  `json:"author"`
	Text    string  `json:"text"`
	ID      string  `json:"id"`
	IP      net.IP  `json:"ip"`
	Referer string  `json:"referer"`
}

type ArtPlayerInput struct {
	Text    string  `json:"text"`
	Time    float32 `json:"time"`
	Color   string  `json:"color"`
	Size    int     `json:"size"`
	Border  bool    `json:"border"`
	Mode    int     `json:"mode"`
	ID      string  `json:"id"`
	IP      net.IP  `json:"ip"`
	Referer string  `json:"referer"`
}

type ArtPlayerData struct {
	Text   *string `json:"text"`
	Time   float32 `json:"time"`
	Color  string  `json:"color"`
	Size   int     `json:"size"`
	Border bool    `json:"border"`
	Mode   int     `json:"mode"`
}

type Referer struct {
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Path     string `json:"path"`
	Query    string `json:"query"`
	Fragment string `json:"fragment"`
}

type Video struct {
	ThirdPartyDanmakuCount int               `json:"thirdPartyDanmakuCount"`
	ID                     int               `json:"id"`
	Vid                    string            `json:"vid"`
	Name                   string            `json:"name"`
	IsDeleted              bool              `json:"isDelete"`
	DefaultPool            bool              `json:"defaultPool"`
	DanmakuCount           int               `json:"danmakuCount"`
	BilibiliPoolCount      int               `json:"bilibiliPoolCount"`
	BilibiliBindings       []BilibiliBinding `json:"bilibiliBindings,omitempty"`
	IqiyiPoolCount         int               `json:"iqiyiPoolCount"`
	IqiyiBindings          []IqiyiBinding    `json:"iqiyiBindings,omitempty"`
	ExternalPoolCount      int               `json:"externalPoolCount"`
	ExternalBindings       []ExternalBinding `json:"externalBindings,omitempty"`
	Referer                *Referer          `json:"referer,omitempty"`
	CreateTime             time.Time         `json:"createTime"`
	UpdateTime             time.Time         `json:"updateTime"`
}

// DanmakuVideo preserves the video object embedded in the existing danmaku
// management response. New video-management endpoints use Video instead.
type DanmakuVideo struct {
	ID         int       `json:"id"`
	Vid        string    `json:"vid"`
	Referer    Referer   `json:"referer"`
	CreateTime time.Time `json:"createTime"`
	UpdateTime time.Time `json:"upDateTime"`
}

type Danmaku struct {
	ID         string        `json:"id"`
	Vid        string        `json:"vid"`
	Data       DanmakuData   `json:"data"`
	IP         net.IP        `json:"ip"`
	IsDeleted  bool          `json:"isDelete"`
	Video      *DanmakuVideo `json:"video"`
	CreateTime time.Time     `json:"createTime"`
	UpdateTime time.Time     `json:"updateTime"`
}

type User struct {
	ID             int       `json:"id"`
	Name           string    `json:"name"`
	Password       *string   `json:"passWord"`
	Salt           string    `json:"salt"`
	Role           int       `json:"role"`
	Enabled        bool      `json:"-"`
	PhoneNumber    *string   `json:"phoneNumber"`
	Email          *string   `json:"email"`
	CreateTime     time.Time `json:"createTime"`
	UpdateTime     time.Time `json:"updateTime"`
	CASSubject     *string   `json:"-"`
	CASDisplayName *string   `json:"-"`
	CASAvatar      *string   `json:"-"`
}

type CASProfile struct {
	Subject     string
	UserName    string
	Email       string
	DisplayName string
	Avatar      string
}

type Page[T any] struct {
	Total int `json:"total"`
	List  []T `json:"list"`
}

type BilibiliPool struct {
	ID              int        `json:"id"`
	BVID            string     `json:"bvid"`
	AID             int64      `json:"aid"`
	Page            int        `json:"p"`
	CID             int64      `json:"cid"`
	DanmakuCount    int        `json:"danmakuCount"`
	BlockedCount    int        `json:"blockedCount"`
	BindingCount    int        `json:"bindingCount"`
	LastAttemptTime *time.Time `json:"lastAttemptTime"`
	LastSyncTime    *time.Time `json:"lastSyncTime"`
	CreateTime      time.Time  `json:"createTime"`
	UpdateTime      time.Time  `json:"updateTime"`
}

type BilibiliDanmaku struct {
	ID              int64       `json:"id"`
	PoolID          int         `json:"poolId"`
	Data            DanmakuData `json:"data"`
	IsBlocked       bool        `json:"isBlocked"`
	ManuallyBlocked bool        `json:"manuallyBlocked"`
	CreateTime      time.Time   `json:"createTime"`
	UpdateTime      time.Time   `json:"updateTime"`
}

type BilibiliKeyword struct {
	ID         int       `json:"id"`
	PoolID     *int      `json:"poolId"`
	PoolBVID   string    `json:"poolBvid"`
	PoolAID    int64     `json:"poolAid"`
	PoolPage   int       `json:"poolP"`
	PoolCID    int64     `json:"poolCid"`
	Keyword    string    `json:"keyword"`
	CreateTime time.Time `json:"createTime"`
}

type BilibiliBinding struct {
	ID         int       `json:"id"`
	Vid        string    `json:"vid"`
	PoolID     int       `json:"poolId"`
	BVID       string    `json:"bvid"`
	AID        int64     `json:"aid"`
	Page       int       `json:"p"`
	CID        int64     `json:"cid"`
	Offset     float64   `json:"offset"`
	CreateTime time.Time `json:"createTime"`
	UpdateTime time.Time `json:"updateTime"`
}

type IqiyiPool struct {
	ID              int        `json:"id"`
	VID             string     `json:"vid"`
	DanmakuCount    int        `json:"danmakuCount"`
	BlockedCount    int        `json:"blockedCount"`
	BindingCount    int        `json:"bindingCount"`
	LastAttemptTime *time.Time `json:"lastAttemptTime"`
	LastSyncTime    *time.Time `json:"lastSyncTime"`
	CreateTime      time.Time  `json:"createTime"`
	UpdateTime      time.Time  `json:"updateTime"`
}

type IqiyiDanmaku struct {
	ID              int64       `json:"id"`
	PoolID          int         `json:"poolId"`
	Data            DanmakuData `json:"data"`
	IsBlocked       bool        `json:"isBlocked"`
	ManuallyBlocked bool        `json:"manuallyBlocked"`
	CreateTime      time.Time   `json:"createTime"`
	UpdateTime      time.Time   `json:"updateTime"`
}

type IqiyiKeyword struct {
	ID         int       `json:"id"`
	PoolID     *int      `json:"poolId"`
	PoolVID    string    `json:"poolVid"`
	Keyword    string    `json:"keyword"`
	CreateTime time.Time `json:"createTime"`
}

type IqiyiBinding struct {
	ID         int       `json:"id"`
	Vid        string    `json:"vid"`
	PoolID     int       `json:"poolId"`
	PoolVID    string    `json:"poolVid"`
	Offset     float64   `json:"offset"`
	CreateTime time.Time `json:"createTime"`
	UpdateTime time.Time `json:"updateTime"`
}

type ExternalPool struct {
	BlockedCount int               `json:"blockedCount"`
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	SourceFormat string            `json:"sourceFormat"`
	DanmakuCount int               `json:"danmakuCount"`
	BindingCount int               `json:"bindingCount"`
	Bindings     []ExternalBinding `json:"bindings,omitempty"`
	CreateTime   time.Time         `json:"createTime"`
	UpdateTime   time.Time         `json:"updateTime"`
}

type ExternalDanmaku struct {
	KeywordBlocked bool        `json:"keywordBlocked"`
	ID             int64       `json:"id"`
	PoolID         string      `json:"poolId"`
	Data           DanmakuData `json:"data"`
	CreateTime     time.Time   `json:"createTime"`
	UpdateTime     time.Time   `json:"updateTime"`
}

type ExternalKeyword struct {
	ID         int       `json:"id"`
	PoolID     *string   `json:"poolId"`
	PoolName   string    `json:"poolName"`
	Keyword    string    `json:"keyword"`
	CreateTime time.Time `json:"createTime"`
}

type ExternalBinding struct {
	ID         int       `json:"id"`
	Vid        string    `json:"vid"`
	PoolID     string    `json:"poolId"`
	PoolName   string    `json:"poolName"`
	Offset     float64   `json:"offset"`
	CreateTime time.Time `json:"createTime"`
	UpdateTime time.Time `json:"updateTime"`
}

type DBData struct {
	Time      float32 `json:"Time"`
	Mode      int     `json:"Mode"`
	Size      int     `json:"Size"`
	Color     int     `json:"Color"`
	Timestamp int64   `json:"TimeStamp"`
	Pool      int     `json:"Pool"`
	Author    string  `json:"Author"`
	AuthorID  int     `json:"AuthorId"`
	Text      *string `json:"Text"`
}

func MarshalDBData(data DanmakuData) ([]byte, error) {
	return json.Marshal(DBData{
		Time: data.Time, Mode: data.Mode, Size: data.Size, Color: data.Color,
		Timestamp: data.Timestamp, Pool: data.Pool, Author: data.Author,
		AuthorID: data.AuthorID, Text: data.Text,
	})
}

type DBReferer struct {
	Protocol string `json:"Protocol"`
	Host     string `json:"Host"`
	Port     int    `json:"Port"`
	Path     string `json:"Path"`
	Query    string `json:"Query"`
	Fragment string `json:"Fragment"`
}

func MarshalDBReferer(value Referer) ([]byte, error) {
	return json.Marshal(DBReferer(value))
}
