package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	KestrelSettings  ListenerSettings  `json:"KestrelSettings" yaml:"KestrelSettings"`
	WithOrigins      []string          `json:"WithOrigins" yaml:"WithOrigins"`
	LiveWithOrigins  []string          `json:"LiveWithOrigins" yaml:"LiveWithOrigins"`
	AdminWithOrigins []string          `json:"AdminWithOrigins" yaml:"AdminWithOrigins"`
	DanmakuSQL       DatabaseSettings  `json:"DanmakuSql" yaml:"DanmakuSql"`
	LegacyDanmakuSQL *DatabaseSettings `json:"DanmuSql,omitempty" yaml:"DanmuSql,omitempty"`
	Admin            AdminSettings     `json:"Admin" yaml:"Admin"`
	Bilibili         BilibiliSettings  `json:"BiliBiliSetting" yaml:"BiliBiliSetting"`
	CAS              CASSettings       `json:"CAS" yaml:"CAS"`
}

type ListenerSettings struct {
	Host           string `json:"Host" yaml:"Host"`
	Port           int    `json:"Port" yaml:"Port"`
	UnixSocketPath string `json:"UnixSocketPath" yaml:"UnixSocketPath"`
}

type DatabaseSettings struct {
	Host     string `json:"Host" yaml:"Host"`
	Port     int    `json:"Port" yaml:"Port"`
	UserName string `json:"UserName" yaml:"UserName"`
	Password string `json:"PassWord" yaml:"PassWord"`
	Database string `json:"DataBase" yaml:"DataBase"`
	PoolSize int32  `json:"PoolSize" yaml:"PoolSize"`
}

type AdminSettings struct {
	User     string `json:"User" yaml:"User"`
	Password string `json:"Password" yaml:"Password"`
	MaxAge   int    `json:"MaxAge" yaml:"MaxAge"`
}

type BilibiliSettings struct {
	Cookie                 string `json:"Cookie" yaml:"Cookie"`
	CIDCacheMinutes        int    `json:"CidCacheTime" yaml:"CidCacheTime"`
	DataCacheMinutes       int    `json:"DanmakuCacheTime" yaml:"DanmakuCacheTime"`
	LegacyDanmakuCacheTime *int   `json:"DanmuCacheTime,omitempty" yaml:"DanmuCacheTime,omitempty"`
}

type CASSettings struct {
	Enabled               bool   `json:"Enabled" yaml:"Enabled"`
	DefaultLogin          bool   `json:"DefaultLogin" yaml:"DefaultLogin"`
	BaseURL               string `json:"BaseURL" yaml:"BaseURL"`
	ValidationURL         string `json:"ValidationURL" yaml:"ValidationURL"`
	ValidationHost        string `json:"ValidationHost" yaml:"ValidationHost"`
	PublicURL             string `json:"PublicURL" yaml:"PublicURL"`
	AutoCreateUsers       bool   `json:"AutoCreateUsers" yaml:"AutoCreateUsers"`
	DefaultRole           int    `json:"DefaultRole" yaml:"DefaultRole"`
	SessionMaxAgeMinutes  int    `json:"SessionMaxAge" yaml:"SessionMaxAge"`
	RequestTimeoutSeconds int    `json:"RequestTimeout" yaml:"RequestTimeout"`
	CookieSecure          bool   `json:"CookieSecure" yaml:"CookieSecure"`
}

func defaults() Config {
	return Config{
		KestrelSettings: ListenerSettings{Host: "127.0.0.1", Port: 80},
		DanmakuSQL:      DatabaseSettings{Host: "127.0.0.1", Port: 5432, PoolSize: 8},
		Admin:           AdminSettings{MaxAge: 1},
		Bilibili:        BilibiliSettings{CIDCacheMinutes: 72, DataCacheMinutes: 5},
		CAS: CASSettings{
			DefaultLogin: true, AutoCreateUsers: true, DefaultRole: 1,
			SessionMaxAgeMinutes:  7 * 24 * 60,
			RequestTimeoutSeconds: 10, CookieSecure: true,
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
		return Config{}, errors.New("KestrelSettings.Port and UnixSocketPath cannot both be empty")
	}
	if cfg.DanmakuSQL.Port == 0 {
		cfg.DanmakuSQL.Port = 5432
	}
	if cfg.DanmakuSQL.PoolSize <= 0 {
		cfg.DanmakuSQL.PoolSize = 8
	}
	if cfg.Admin.MaxAge <= 0 {
		cfg.Admin.MaxAge = 1
	}
	if cfg.CAS.SessionMaxAgeMinutes <= 0 {
		cfg.CAS.SessionMaxAgeMinutes = 7 * 24 * 60
	}
	if cfg.CAS.RequestTimeoutSeconds <= 0 {
		cfg.CAS.RequestTimeoutSeconds = 10
	}
	if cfg.CAS.Enabled {
		if cfg.CAS.DefaultRole != 1 && cfg.CAS.DefaultRole != 2 {
			return Config{}, errors.New("CAS.DefaultRole must be 1 (SuperAdmin) or 2 (Admin)")
		}
		if err := validateAbsoluteURL(cfg.CAS.BaseURL, "CAS.BaseURL"); err != nil {
			return Config{}, err
		}
		if err := validateAbsoluteURL(cfg.CAS.PublicURL, "CAS.PublicURL"); err != nil {
			return Config{}, err
		}
		if cfg.CAS.ValidationURL != "" {
			if err := validateAbsoluteURL(cfg.CAS.ValidationURL, "CAS.ValidationURL"); err != nil {
				return Config{}, err
			}
		}
	}
	return cfg, nil
}

func configCandidates() []string {
	result := []string{"appsettings.json", "appsettings.yml"}
	environment := os.Getenv("ASPNETCORE_ENVIRONMENT")
	if environment == "" {
		environment = os.Getenv("DOTNET_ENVIRONMENT")
	}
	if environment != "" {
		result = append(result, "appsettings."+environment+".json", "appsettings."+environment+".yml")
	}
	return result
}

func mergeFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config %q: %w", path, err)
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		err = json.Unmarshal(data, cfg)
	case ".yml", ".yaml":
		err = yaml.Unmarshal(data, cfg)
	default:
		return fmt.Errorf("unsupported config format %q", filepath.Ext(path))
	}
	if err != nil {
		return fmt.Errorf("parse config %q: %w", path, err)
	}
	if cfg.LegacyDanmakuSQL != nil {
		cfg.DanmakuSQL = *cfg.LegacyDanmakuSQL
		cfg.LegacyDanmakuSQL = nil
	}
	if cfg.Bilibili.LegacyDanmakuCacheTime != nil {
		cfg.Bilibili.DataCacheMinutes = *cfg.Bilibili.LegacyDanmakuCacheTime
		cfg.Bilibili.LegacyDanmakuCacheTime = nil
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
