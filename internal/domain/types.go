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
	ID         int       `json:"id"`
	Vid        string    `json:"vid"`
	Referer    Referer   `json:"referer"`
	CreateTime time.Time `json:"createTime"`
	UpdateTime time.Time `json:"upDateTime"`
}

type Danmaku struct {
	ID         string      `json:"id"`
	Vid        string      `json:"vid"`
	Data       DanmakuData `json:"data"`
	IP         net.IP      `json:"ip"`
	IsDeleted  bool        `json:"isDelete"`
	Video      *Video      `json:"video"`
	CreateTime time.Time   `json:"createTime"`
	UpdateTime time.Time   `json:"updateTime"`
}

type User struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Password    *string   `json:"passWord"`
	Salt        string    `json:"salt"`
	Role        int       `json:"role"`
	PhoneNumber *string   `json:"phoneNumber"`
	Email       *string   `json:"email"`
	CreateTime  time.Time `json:"createTime"`
	UpdateTime  time.Time `json:"updateTime"`
}

type Page[T any] struct {
	Total int `json:"total"`
	List  []T `json:"list"`
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
