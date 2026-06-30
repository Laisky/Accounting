// Package config loads runtime settings for the Accounting backend.
package config

import (
	"os"
	"strconv"
	"strings"
)

// Config contains runtime settings for the backend process.
type Config struct {
	Addr       string
	Debug      bool
	Frontend   FrontendConfig
	ServerName string
}

// FrontendConfig contains settings for serving the built React application.
type FrontendConfig struct {
	DistDir string
	DevURL  string
}

// LoadFromEnv reads environment variables and returns a complete runtime configuration.
func LoadFromEnv() Config {
	return Config{
		Addr:       readString("ACCOUNTING_ADDR", ":8080"),
		Debug:      readBool("ACCOUNTING_DEBUG", false),
		ServerName: readString("ACCOUNTING_SERVER_NAME", "accounting"),
		Frontend: FrontendConfig{
			DistDir: readString("ACCOUNTING_WEB_DIST_DIR", "../web/dist"),
			DevURL:  readString("ACCOUNTING_WEB_DEV_URL", ""),
		},
	}
}

// readString returns the trimmed environment value for key or fallback when unset.
func readString(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

// readBool returns the parsed boolean environment value for key or fallback when unset or invalid.
func readBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}
