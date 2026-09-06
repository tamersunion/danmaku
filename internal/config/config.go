package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type DandanplaySettings struct {
	APIBase             string `json:"api_base"`
	SyncIntervalSeconds int    `json:"sync_interval_seconds"`
}

const DefaultDandanplayAPIBase = "https://api.danmaku.weeblify.app/ddp/v1"
const DefaultDandanplaySyncIntervalSeconds = 600

type Config struct {
	Dandanplay       DandanplaySettings `json:"dandanplay_setting"`
	KestrelSettings  ListenerSettings   `json:"kestrel_settings"`
	WithOrigins      []string           `json:"with_origins"`
	LiveWithOrigins  []string           `json:"live_with_origins"`
	AdminWithOrigins []string           `json:"admin_with_origins"`
	DanmakuSQL       DatabaseSettings   `json:"danmaku_sql"`
	Admin            AdminSettings      `json:"admin"`
	Bilibili         BilibiliSettings   `json:"bilibili_setting"`
	Iqiyi            IqiyiSettings      `json:"iqiyi_setting"`
	Redis            RedisSettings      `json:"redis"`
	CAS              CASSettings        `json:"cas"`
}

type ListenerSettings struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	UnixSocketPath string `json:"unix_socket_path"`
}

type DatabaseSettings struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	UserName string `json:"user_name"`
	Password string `json:"password"`
	Database string `json:"database"`
	PoolSize int32  `json:"pool_size"`
}

type AdminSettings struct {
	User          string `json:"user"`
	Password      string `json:"password"`
	MaxAgeSeconds int    `json:"max_age_seconds"`
}

type BilibiliSettings struct {
	Cookie              string `json:"cookie"`
	APIBase             string `json:"api_base"`
	CIDCacheSeconds     int    `json:"cid_cache_seconds"`
	SyncIntervalSeconds int    `json:"sync_interval_seconds"`
}

type IqiyiSettings struct {
	DecodeAPIBase       string `json:"decode_api_base"`
	VideoInfoAPIBase    string `json:"video_info_api_base"`
	DanmakuAPIBase      string `json:"danmaku_api_base"`
	SyncIntervalSeconds int    `json:"sync_interval_seconds"`
}

type RedisSettings struct {
	Enabled    bool   `json:"enabled"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Password   string `json:"password"`
	Database   int    `json:"database"`
	KeyPrefix  string `json:"key_prefix"`
	TTLSeconds int    `json:"ttl_seconds"`
}

const (
	DefaultBilibiliAPIBase             = "https://api.bilibili.com"
	DefaultBilibiliCIDCacheSeconds     = 72 * 60
	DefaultBilibiliSyncIntervalSeconds = 10 * 60
	DefaultIqiyiDecodeAPIBase          = "https://pcw-api.iq.com/api/decode"
	DefaultIqiyiVideoInfoAPIBase       = "https://pcw-api.iqiyi.com/video/video/baseinfo"
	DefaultIqiyiDanmakuAPIBase         = "https://cmts.iqiyi.com/bullet"
	DefaultIqiyiSyncIntervalSeconds    = 10 * 60
	DefaultRedisTTLSeconds             = 60 * 60
	DefaultCASSessionMaxAgeSeconds     = 7 * 24 * 60 * 60
	DefaultCASRequestTimeoutSeconds    = 10
	DefaultAdminSessionMaxAgeSeconds   = 60
)

type CASSettings struct {
	Enabled               bool   `json:"enabled"`
	DefaultLogin          bool   `json:"default_login"`
	BaseURL               string `json:"base_url"`
	ValidationURL         string `json:"validation_url"`
	ValidationHost        string `json:"validation_host"`
	PublicURL             string `json:"public_url"`
	AutoCreateUsers       bool   `json:"auto_create_users"`
	DefaultRole           int    `json:"default_role"`
	SessionMaxAgeSeconds  int    `json:"session_max_age_seconds"`
	RequestTimeoutSeconds int    `json:"request_timeout_seconds"`
	CookieSecure          bool   `json:"cookie_secure"`
}

func defaults() Config {
	return Config{
		KestrelSettings: ListenerSettings{Host: "127.0.0.1", Port: 80},
		DanmakuSQL:      DatabaseSettings{Host: "127.0.0.1", Port: 5432, PoolSize: 8},
		Admin:           AdminSettings{MaxAgeSeconds: DefaultAdminSessionMaxAgeSeconds},
		Bilibili: BilibiliSettings{
			APIBase: DefaultBilibiliAPIBase, CIDCacheSeconds: DefaultBilibiliCIDCacheSeconds,
			SyncIntervalSeconds: DefaultBilibiliSyncIntervalSeconds,
		},
		Dandanplay: DandanplaySettings{APIBase: DefaultDandanplayAPIBase, SyncIntervalSeconds: DefaultDandanplaySyncIntervalSeconds},
		Iqiyi: IqiyiSettings{
			DecodeAPIBase: DefaultIqiyiDecodeAPIBase, VideoInfoAPIBase: DefaultIqiyiVideoInfoAPIBase,
			DanmakuAPIBase: DefaultIqiyiDanmakuAPIBase, SyncIntervalSeconds: DefaultIqiyiSyncIntervalSeconds,
		},
		Redis: RedisSettings{Host: "127.0.0.1", Port: 6379, KeyPrefix: "danmaku", TTLSeconds: DefaultRedisTTLSeconds},
		CAS: CASSettings{
			DefaultLogin: true, AutoCreateUsers: true, DefaultRole: 3,
			SessionMaxAgeSeconds:  DefaultCASSessionMaxAgeSeconds,
			RequestTimeoutSeconds: DefaultCASRequestTimeoutSeconds, CookieSecure: true,
		},
	}
}

func Load(path string) (Config, error) {
	cfg := defaults()
	if path == "" {
		path = os.Getenv("DANMAKU_CONFIG")
	}
	if path != "" {
		if err := mergeFile(path, &cfg); err != nil {
			return Config{}, err
		}
	} else {
		for _, candidate := range configCandidates() {
			if _, err := os.Stat(candidate); err == nil {
				if err := mergeFile(candidate, &cfg); err != nil {
					return Config{}, err
				}
			}
		}
	}

	if cfg.KestrelSettings.Host == "" {
		cfg.KestrelSettings.Host = "127.0.0.1"
	}
	if cfg.KestrelSettings.Port == 0 && cfg.KestrelSettings.UnixSocketPath == "" {
		return Config{}, errors.New("kestrel_settings.port and unix_socket_path cannot both be empty")
	}
	if cfg.DanmakuSQL.Port == 0 {
		cfg.DanmakuSQL.Port = 5432
	}
	if cfg.DanmakuSQL.PoolSize <= 0 {
		cfg.DanmakuSQL.PoolSize = 8
	}
	if cfg.Admin.MaxAgeSeconds <= 0 {
		cfg.Admin.MaxAgeSeconds = DefaultAdminSessionMaxAgeSeconds
	}
	cfg.Bilibili.APIBase = strings.TrimRight(strings.TrimSpace(cfg.Bilibili.APIBase), "/")
	if cfg.Bilibili.APIBase == "" {
		cfg.Bilibili.APIBase = DefaultBilibiliAPIBase
	}
	if err := validateAbsoluteURL(cfg.Bilibili.APIBase, "bilibili_setting.api_base"); err != nil {
		return Config{}, err
	}
	if cfg.Bilibili.CIDCacheSeconds <= 0 {
		return Config{}, errors.New("bilibili_setting.cid_cache_seconds must be greater than zero")
	}
	if cfg.Bilibili.SyncIntervalSeconds <= 0 {
		return Config{}, errors.New("bilibili_setting.sync_interval_seconds must be greater than zero")
	}
	cfg.Iqiyi.DecodeAPIBase = strings.TrimRight(strings.TrimSpace(cfg.Iqiyi.DecodeAPIBase), "/")
	cfg.Iqiyi.VideoInfoAPIBase = strings.TrimRight(strings.TrimSpace(cfg.Iqiyi.VideoInfoAPIBase), "/")
	cfg.Iqiyi.DanmakuAPIBase = strings.TrimRight(strings.TrimSpace(cfg.Iqiyi.DanmakuAPIBase), "/")
	if cfg.Iqiyi.DecodeAPIBase == "" {
		cfg.Iqiyi.DecodeAPIBase = DefaultIqiyiDecodeAPIBase
	}
	if cfg.Iqiyi.VideoInfoAPIBase == "" {
		cfg.Iqiyi.VideoInfoAPIBase = DefaultIqiyiVideoInfoAPIBase
	}
	if cfg.Iqiyi.DanmakuAPIBase == "" {
		cfg.Iqiyi.DanmakuAPIBase = DefaultIqiyiDanmakuAPIBase
	}
	for name, value := range map[string]string{
		"iqiyi_setting.decode_api_base":     cfg.Iqiyi.DecodeAPIBase,
		"iqiyi_setting.video_info_api_base": cfg.Iqiyi.VideoInfoAPIBase,
		"iqiyi_setting.danmaku_api_base":    cfg.Iqiyi.DanmakuAPIBase,
	} {
		if err := validateAbsoluteURL(value, name); err != nil {
			return Config{}, err
		}
	}
	if cfg.Iqiyi.SyncIntervalSeconds <= 0 {
		return Config{}, errors.New("iqiyi_setting.sync_interval_seconds must be greater than zero")
	}
	cfg.Dandanplay.APIBase = strings.TrimSpace(cfg.Dandanplay.APIBase)
	if cfg.Dandanplay.APIBase == "" {
		cfg.Dandanplay.APIBase = DefaultDandanplayAPIBase
	}
	if err := validateAbsoluteURL(cfg.Dandanplay.APIBase, "dandanplay_setting.api_base"); err != nil {
		return Config{}, err
	}
	if cfg.Dandanplay.SyncIntervalSeconds <= 0 {
		return Config{}, errors.New("dandanplay_setting.sync_interval_seconds must be greater than zero")
	}
	if cfg.Redis.Host == "" {
		cfg.Redis.Host = "127.0.0.1"
	}
	if cfg.Redis.Port == 0 {
		cfg.Redis.Port = 6379
	}
	if cfg.Redis.Port < 1 || cfg.Redis.Port > 65535 {
		return Config{}, errors.New("redis.port must be between 1 and 65535")
	}
	if cfg.Redis.Database < 0 {
		return Config{}, errors.New("redis.database cannot be negative")
	}
	cfg.Redis.KeyPrefix = strings.Trim(strings.TrimSpace(cfg.Redis.KeyPrefix), ":")
	if cfg.Redis.KeyPrefix == "" {
		cfg.Redis.KeyPrefix = "danmaku"
	}
	if cfg.Redis.TTLSeconds <= 0 {
		return Config{}, errors.New("redis.ttl_seconds must be greater than zero")
	}
	if cfg.CAS.SessionMaxAgeSeconds <= 0 {
		cfg.CAS.SessionMaxAgeSeconds = DefaultCASSessionMaxAgeSeconds
	}
	if cfg.CAS.RequestTimeoutSeconds <= 0 {
		cfg.CAS.RequestTimeoutSeconds = DefaultCASRequestTimeoutSeconds
	}
	if cfg.CAS.Enabled {
		if cfg.CAS.DefaultRole != 1 && cfg.CAS.DefaultRole != 2 && cfg.CAS.DefaultRole != 3 {
			return Config{}, errors.New("cas.default_role must be 1 (SuperAdmin), 2 (Admin), or 3 (GeneralUser)")
		}
		if err := validateAbsoluteURL(cfg.CAS.BaseURL, "cas.base_url"); err != nil {
			return Config{}, err
		}
		if err := validateAbsoluteURL(cfg.CAS.PublicURL, "cas.public_url"); err != nil {
			return Config{}, err
		}
		if cfg.CAS.ValidationURL != "" {
			if err := validateAbsoluteURL(cfg.CAS.ValidationURL, "cas.validation_url"); err != nil {
				return Config{}, err
			}
		}
	}
	return cfg, nil
}

func configCandidates() []string {
	result := []string{"appsettings.json"}
	environment := strings.TrimSpace(os.Getenv("DANMAKU_ENVIRONMENT"))
	if environment != "" {
		result = append(result, "appsettings."+environment+".json")
	}
	return result
}

func mergeFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config %q: %w", path, err)
	}
	if strings.ToLower(filepath.Ext(path)) != ".json" {
		return fmt.Errorf("unsupported config format %q", filepath.Ext(path))
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(cfg)
	if err == nil && len(bytes.TrimSpace(data[decoder.InputOffset():])) != 0 {
		err = errors.New("unexpected data after JSON document")
	}
	if err != nil {
		return fmt.Errorf("parse config %q: %w", path, err)
	}
	return nil
}

func validateAbsoluteURL(raw, name string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("%s must be an absolute URL", name)
	}
	if parsed.Scheme != "https" && parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" {
		return fmt.Errorf("%s must use HTTPS", name)
	}
	return nil
}

func (d DatabaseSettings) ConnectionString() string {
	value := url.URL{
		Scheme:  "postgres",
		User:    url.UserPassword(d.UserName, d.Password),
		Host:    net.JoinHostPort(d.Host, strconv.Itoa(d.Port)),
		Path:    "/" + d.Database,
		RawPath: "/" + url.PathEscape(d.Database),
	}
	return value.String()
}
