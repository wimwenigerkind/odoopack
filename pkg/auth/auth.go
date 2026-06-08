package auth

import (
	"encoding/json"
	"net/url"

	"github.com/spf13/viper"
)

type authConfig struct {
	Bearer map[string]string `json:"bearer"`
}

func TokenForURL(rawURL string) string {
	raw := viper.GetString("auth")
	if raw == "" {
		return ""
	}
	var cfg authConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return ""
	}
	return cfg.Bearer[u.Host]
}
