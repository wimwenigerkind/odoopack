package auth

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func setAuth(t *testing.T, value string) {
	t.Helper()
	viper.Set("auth", value)
	t.Cleanup(viper.Reset)
}

func TestTokenForURLMatchesHostAndOrigin(t *testing.T) {
	for _, value := range []string{
		`{"bearer":{"registry.example.com":"secret"}}`,
		`{"bearer":{"https://registry.example.com":"secret"}}`,
	} {
		setAuth(t, value)
		got, err := TokenForURL("https://registry.example.com/registry/v1/addons/acme/health-check")
		if err != nil || got != "secret" {
			t.Errorf("%s: got %q, %v", value, got, err)
		}
	}
}

func TestTokenForURLWithoutAuth(t *testing.T) {
	setAuth(t, "")
	if got, err := TokenForURL("https://registry.example.com/"); got != "" || err != nil {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestTokenForURLRejectsMalformedAuth(t *testing.T) {
	for _, value := range []string{
		`registry.example.com=secret`,
		`{"bearer":{"registry.example.com":"secret"}`,
		`{"baerer":{"registry.example.com":"secret"}}`,
		`{"bearer":["secret"]}`,
		`{"bearer":{}} trailing`,
	} {
		setAuth(t, value)
		_, err := TokenForURL("https://registry.example.com/")
		if err == nil || !strings.Contains(err.Error(), "ODOOPACK_AUTH") {
			t.Errorf("%s: got error %v, want one naming ODOOPACK_AUTH", value, err)
		}
		if err != nil && strings.Contains(err.Error(), "secret") {
			t.Errorf("%s: error %v reveals the token", value, err)
		}
	}
}
