package main

import (
	"fmt"
	"os"
	"strconv"
)

type TenantMode string

const (
	ModeSafe   TenantMode = "safe"   // per-request transaction, set_config(..., is_local=true)
	ModeUnsafe TenantMode = "unsafe" // session-level set_config on the pool: the blog's leak demo
)

type Config struct {
	ListenAddr   string
	AppURL       string // lineview_app: tenant data
	AuthURL      string // lineview_auth: auth.sessions only
	AppMaxConns  int32
	AuthMaxConns int32
	Mode         TenantMode
}

func loadConfig() (Config, error) {
	c := Config{
		ListenAddr:   getenv("LISTEN_ADDR", ":8080"),
		AppURL:       os.Getenv("APP_DATABASE_URL"),
		AuthURL:      os.Getenv("AUTH_DATABASE_URL"),
		AppMaxConns:  int32(getenvInt("APP_MAX_CONNS", 10)),
		AuthMaxConns: int32(getenvInt("AUTH_MAX_CONNS", 4)),
		Mode:         TenantMode(getenv("TENANT_MODE", string(ModeSafe))),
	}

	if c.AppURL == "" || c.AuthURL == "" {
		return c, fmt.Errorf("APP_DATABASE_URL and AUTH_DATABASE_URL are required")
	}
	if c.Mode != ModeSafe && c.Mode != ModeUnsafe {
		return c, fmt.Errorf("TENANT_MODE must be %q or %q, got %q", ModeSafe, ModeUnsafe, c.Mode)
	}
	return c, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getenvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
