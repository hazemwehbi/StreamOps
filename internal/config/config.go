package config

import (
	"fmt"
	"math"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address         string
	DatabaseURL     string
	AdminToken      string
	ReadToken       string
	CORSOrigin      string
	MetricsToken    string
	OTLPEndpoint    string
	PoolMax         int32
	DBTimeout       time.Duration
	ShutdownTimeout time.Duration
	Rate            float64
	Burst           int
}

func Load() (c Config, err error) {
	c = Config{Address: env("HTTP_ADDR", ":8080"), DatabaseURL: os.Getenv("DATABASE_URL"), AdminToken: os.Getenv("ADMIN_TOKEN"), ReadToken: os.Getenv("READ_TOKEN"), MetricsToken: os.Getenv("METRICS_TOKEN"), CORSOrigin: os.Getenv("CORS_ORIGIN"), OTLPEndpoint: os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"), PoolMax: 10, DBTimeout: 5 * time.Second, ShutdownTimeout: 15 * time.Second, Rate: 100, Burst: 200}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL est obligatoire")
	}
	if len(c.AdminToken) < 32 || len(c.ReadToken) < 32 || len(c.MetricsToken) < 32 {
		return c, fmt.Errorf("ADMIN_TOKEN, READ_TOKEN et METRICS_TOKEN doivent chacun contenir au moins 32 octets")
	}
	if c.AdminToken == c.ReadToken || c.AdminToken == c.MetricsToken || c.ReadToken == c.MetricsToken {
		return c, fmt.Errorf("les jetons doivent être distincts")
	}
	if c.CORSOrigin == "*" || strings.ContainsAny(c.CORSOrigin, "\r\n") {
		return c, fmt.Errorf("CORS_ORIGIN doit définir une seule origine explicite")
	}
	if c.CORSOrigin != "" {
		origin, e := url.Parse(c.CORSOrigin)
		if e != nil || !((origin.Scheme == "https" || origin.Scheme == "http") && origin.Host != "" && origin.Path == "" && origin.RawQuery == "" && origin.Fragment == "" && origin.User == nil) {
			return c, fmt.Errorf("CORS_ORIGIN doit être une origine http(s) sans chemin")
		}
	}
	if v := os.Getenv("DB_POOL_MAX"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 || n > 100 {
			return c, fmt.Errorf("DB_POOL_MAX doit être compris entre 1 et 100")
		}
		c.PoolMax = int32(n)
	}
	if v := os.Getenv("DB_TIMEOUT"); v != "" {
		c.DBTimeout, err = time.ParseDuration(v)
		if err != nil || c.DBTimeout < time.Millisecond || c.DBTimeout > time.Minute {
			return c, fmt.Errorf("DB_TIMEOUT doit être compris entre 1ms et 1m")
		}
	}
	if v := os.Getenv("RATE_LIMIT_RPS"); v != "" {
		c.Rate, err = strconv.ParseFloat(v, 64)
		if err != nil || math.IsNaN(c.Rate) || c.Rate <= 0 || c.Rate > 100000 {
			return c, fmt.Errorf("RATE_LIMIT_RPS invalide")
		}
	}
	if v := os.Getenv("RATE_LIMIT_BURST"); v != "" {
		c.Burst, err = strconv.Atoi(v)
		if err != nil || c.Burst < 1 || c.Burst > 100000 {
			return c, fmt.Errorf("RATE_LIMIT_BURST invalide")
		}
	}
	return c, nil
}
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
