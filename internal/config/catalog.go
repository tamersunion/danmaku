package config

import (
	"fmt"
	"strings"
)

type CatalogSettings struct {
	APIBase             string `json:"api_base"`
	SearchAPIBase       string `json:"search_api_base"`
	EpisodesAPIBase     string `json:"episodes_api_base"`
	VideoInfoAPIBase    string `json:"video_info_api_base,omitempty"`
	SessionAPIBase      string `json:"session_api_base,omitempty"`
	CNAAPIBase          string `json:"cna_api_base,omitempty"`
	Cookie              string `json:"cookie,omitempty"`
	SyncIntervalSeconds int    `json:"sync_interval_seconds"`
}

func DefaultCatalogSettings(source string) CatalogSettings {
	c := CatalogSettings{SyncIntervalSeconds: 600}
	switch source {
	case "bahamut":
		c.APIBase = "https://api.gamer.com.tw/anime/v1/danmu.php"
		c.SearchAPIBase = "https://api.gamer.com.tw/mobile_app/anime/v1/search.php"
		c.EpisodesAPIBase = "https://api.gamer.com.tw/anime/v1/video.php"
	case "tencent":
		c.APIBase = "https://dm.video.qq.com/barrage"
		c.SearchAPIBase = "https://pbaccess.video.qq.com/trpc.videosearch.mobile_search.MultiTerminalSearch/MbSearch"
		c.EpisodesAPIBase = "https://pbaccess.video.qq.com/trpc.universal_backend_service.page_server_rpc.PageServer/GetPageData"
	case "youku":
		c.APIBase = "https://acs.youku.com/h5/mopen.youku.danmu.list/1.0/"
		c.SearchAPIBase = "https://search.youku.com/api/search"
		c.EpisodesAPIBase = "https://openapi.youku.com/v2/shows/videos.json"
		c.VideoInfoAPIBase = "https://openapi.youku.com/v2/videos/show.json"
		c.SessionAPIBase = "https://acs.youku.com/h5/mtop.com.youku.aplatform.weakget/1.0/"
		c.CNAAPIBase = "https://log.mmstat.com/eg.js"
	}
	return c
}
func (c *CatalogSettings) ApplyDefaults(source string) {
	defaults := DefaultCatalogSettings(source)
	for dst, value := range map[*string]string{&c.APIBase: defaults.APIBase, &c.SearchAPIBase: defaults.SearchAPIBase, &c.EpisodesAPIBase: defaults.EpisodesAPIBase, &c.VideoInfoAPIBase: defaults.VideoInfoAPIBase, &c.SessionAPIBase: defaults.SessionAPIBase, &c.CNAAPIBase: defaults.CNAAPIBase} {
		*dst = strings.TrimSpace(*dst)
		if *dst == "" {
			*dst = value
		}
	}
}
func (c *CatalogSettings) validate(source string) error {
	c.ApplyDefaults(source)
	if c.SyncIntervalSeconds <= 0 {
		return fmt.Errorf("%s_setting.sync_interval_seconds must be greater than zero", source)
	}
	for name, value := range map[string]string{"api_base": c.APIBase, "search_api_base": c.SearchAPIBase, "episodes_api_base": c.EpisodesAPIBase, "video_info_api_base": c.VideoInfoAPIBase, "session_api_base": c.SessionAPIBase, "cna_api_base": c.CNAAPIBase} {
		if value != "" {
			if err := validateAbsoluteURL(value, source+"_setting."+name); err != nil {
				return err
			}
		}
	}
	if strings.ContainsAny(c.Cookie, "\r\n") {
		return fmt.Errorf("%s_setting.cookie contains invalid characters", source)
	}
	return nil
}
