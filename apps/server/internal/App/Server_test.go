package app_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	app "github.com/Arab-OpenSource/dhow/apps/server/internal/App"
	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	support "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/System/Test/Support"
)

func TestNewHTTPServerSetsEveryTimeout(t *testing.T) {
	cfg := config.Config{PublicURL: &url.URL{Scheme: "https", Host: "dhow.example.com"}, ListenAddr: "127.0.0.1:0"}
	srv := app.NewHTTPServer(cfg, http.NotFoundHandler(), support.DiscardLogger())
	tests := []struct {
		name string
		got  any
		want any
	}{
		{"address", srv.Addr, "127.0.0.1:0"},
		{"read header timeout", srv.ReadHeaderTimeout, config.ReadHeaderTimeout},
		{"read timeout", srv.ReadTimeout, config.ReadTimeout},
		{"write timeout", srv.WriteTimeout, config.WriteTimeout},
		{"idle timeout", srv.IdleTimeout, config.IdleTimeout},
		{"max header bytes", srv.MaxHeaderBytes, config.MaxHeaderBytes},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.got)
			assert.NotZero(t, tt.got)
		})
	}
	assert.NotNil(t, srv.ErrorLog)
}
