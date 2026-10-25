package config

import "testing"

func TestConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("ADMIN_TOKEN", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	t.Setenv("READ_TOKEN", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	t.Setenv("METRICS_TOKEN", "cccccccccccccccccccccccccccccccc")
	t.Setenv("DB_POOL_MAX", "")
	t.Setenv("DB_TIMEOUT", "")
	t.Setenv("CORS_ORIGIN", "")
	t.Setenv("RATE_LIMIT_RPS", "")
	t.Setenv("RATE_LIMIT_BURST", "")
	if _, e := Load(); e != nil {
		t.Fatal(e)
	}
	for key, value := range map[string]string{"DB_POOL_MAX": "0", "DB_TIMEOUT": "0s", "CORS_ORIGIN": "*", "RATE_LIMIT_RPS": "NaN", "RATE_LIMIT_BURST": "0", "ADMIN_TOKEN": "short"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, value)
			if _, e := Load(); e == nil {
				t.Fatal("configuration invalide acceptée")
			}
		})
	}
}
