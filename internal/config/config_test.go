package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadValidatesTokenLifetime(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/blog")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("SERVER_PORT", "8080")

	for _, value := range []string{"1ns", "999ms", "0s", "-1s", "invalid"} {
		t.Run("reject_"+value, func(t *testing.T) {
			t.Setenv("JWT_TTL", value)
			if _, err := Load(); err == nil {
				t.Fatalf("Load accepted invalid JWT_TTL %q", value)
			}
		})
	}

	for _, tc := range []struct {
		value string
		want  time.Duration
	}{
		{"1s", time.Second},
		{"24h", 24 * time.Hour},
		{"", 24 * time.Hour},
	} {
		t.Run("accept_"+tc.value, func(t *testing.T) {
			t.Setenv("JWT_TTL", tc.value)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load rejected JWT_TTL %q: %v", tc.value, err)
			}
			if cfg.JWTTTL != tc.want {
				t.Fatalf("got JWT_TTL %v, want %v", cfg.JWTTTL, tc.want)
			}
		})
	}
}
