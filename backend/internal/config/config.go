// Package config reads settings from environment variables (section 11.7 of the guide).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL       string
	DatabaseURLDirect string
	JWTSecret         string
	GeminiAPIKey      string
	GeminiModel       string
	EmbedModel        string
	FieldKey          string
	StorageDir        string
	FrontendOrigin    string
	PublicAPIURL      string // absolute base used in pre-signed file links
	Port              string
	CookieSecure      bool
	LoginRatePerMin   int
	AIRatePerMin      int
	ScamRatePerMin    int
	JobInterval       int // minutes
}

func Load() (*Config, error) {
	_ = godotenv.Load() // .env is optional; real environment variables win
	c := &Config{
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		DatabaseURLDirect: os.Getenv("DATABASE_URL_DIRECT"),
		JWTSecret:         os.Getenv("JWT_SECRET"),
		GeminiAPIKey:      os.Getenv("GEMINI_API_KEY"),
		GeminiModel:       env("GEMINI_MODEL", "gemini-3.5-flash"),
		EmbedModel:        env("EMBED_MODEL", "gemini-embedding-001"),
		FieldKey:          os.Getenv("FIELD_ENCRYPTION_KEY"),
		StorageDir:        env("STORAGE_DIR", "./uploads"),
		FrontendOrigin:    env("FRONTEND_ORIGIN", "http://localhost:3000"),
		Port:              env("PORT", "8080"),
		LoginRatePerMin:   envInt("RATE_LOGIN_PER_MIN", 5),
		AIRatePerMin:      envInt("RATE_AI_PER_MIN", 10),
		ScamRatePerMin:    envInt("RATE_SCAM_PER_MIN", 5),
		JobInterval:       envInt("JOB_INTERVAL_MINUTES", 15),
	}
	c.PublicAPIURL = strings.TrimRight(env("PUBLIC_API_URL", "http://localhost:"+c.Port+"/api/v1"), "/")
	c.CookieSecure = env("COOKIE_SECURE", strconv.FormatBool(strings.HasPrefix(c.FrontendOrigin, "https://"))) == "true"
	if c.DatabaseURLDirect == "" {
		c.DatabaseURLDirect = strings.Replace(c.DatabaseURL, "-pooler", "", 1)
	}
	var missing []string
	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if len(c.JWTSecret) < 32 {
		missing = append(missing, "JWT_SECRET (at least 32 characters)")
	}
	if c.FieldKey == "" {
		missing = append(missing, "FIELD_ENCRYPTION_KEY")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing settings: %s", strings.Join(missing, ", "))
	}
	return c, nil
}

// AIEnabled reports whether a Gemini key is configured. Without it, AI features use rules-only fallbacks.
func (c *Config) AIEnabled() bool { return c.GeminiAPIKey != "" }

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil && v > 0 {
		return v
	}
	return def
}
