package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoad_EnvironmentOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("http:\n  address: 127.0.0.1:8080\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_HTTP_ADDRESS", "127.0.0.1:9090")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTP.Address != "127.0.0.1:9090" {
		t.Fatalf("HTTP.Address = %q, want %q", cfg.HTTP.Address, "127.0.0.1:9090")
	}
}

func TestLoad_EnvironmentStringSlicesAcceptBracketedLists(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("http:\n  address: 127.0.0.1:8080\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(
		"APP_AUTH_PSK_GRPC_METHODS",
		"[/platform.export.v1.ExportProviderService/*, /platform.import.v1.ImportProviderService/*]",
	)
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{
		"/platform.export.v1.ExportProviderService/*",
		"/platform.import.v1.ImportProviderService/*",
	}
	if len(cfg.Auth.PSK.GRPCMethods) != len(want) {
		t.Fatalf("GRPCMethods = %#v", cfg.Auth.PSK.GRPCMethods)
	}
	for index := range want {
		if cfg.Auth.PSK.GRPCMethods[index] != want[index] {
			t.Fatalf("GRPCMethods[%d] = %q, want %q", index, cfg.Auth.PSK.GRPCMethods[index], want[index])
		}
	}
}

func TestLoad_IdempotencyRouteEnvironmentOverrides(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("idempotency:\n  http_paths: [/api/v1/old]\n  grpc_methods: [/old.Service/Create]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_IDEMPOTENCY_HTTP_PATHS", "[/api/v1/plans/create, /api/v1/invoices/generate]")
	t.Setenv("APP_IDEMPOTENCY_GRPC_METHODS", "[/platform.billing.v1.BillingService/CreatePlan]")
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if strings.Join(cfg.Idempotency.HTTPPaths, ",") != "/api/v1/plans/create,/api/v1/invoices/generate" {
		t.Fatalf("HTTPPaths = %#v", cfg.Idempotency.HTTPPaths)
	}
	if strings.Join(cfg.Idempotency.GRPCMethods, ",") != "/platform.billing.v1.BillingService/CreatePlan" {
		t.Fatalf("GRPCMethods = %#v", cfg.Idempotency.GRPCMethods)
	}
}

func TestConfig_ValidateJWTSecret(t *testing.T) {
	t.Parallel()
	cfg := Config{HTTP: HTTP{Address: "127.0.0.1:8080"}, Auth: Auth{ClientID: "client", ClientSecret: "secret"}, JWT: JWT{Secret: "short"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestLoadWithProfile_MergesProfileThenEnvironment(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "config.yaml")
	profile := filepath.Join(dir, "config-test.yaml")
	if err := os.WriteFile(base, []byte("app:\n  env: development\nlog:\n  level: info\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profile, []byte("log:\n  level: debug\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_LOG_LEVEL", "error")
	cfg, err := LoadWithProfile(base, "test")
	if err != nil {
		t.Fatalf("LoadWithProfile() error = %v", err)
	}
	if cfg.App.Env != "test" || cfg.Runtime.ActiveProfile != "test" {
		t.Fatalf("active profile = %q/%q", cfg.App.Env, cfg.Runtime.ActiveProfile)
	}
	if cfg.Log.Level != "error" {
		t.Fatalf("Log.Level = %q, want environment override", cfg.Log.Level)
	}
	if len(cfg.Runtime.ConfigFiles) != 2 || cfg.Runtime.ConfigFiles[1] != profile {
		t.Fatalf("ConfigFiles = %v", cfg.Runtime.ConfigFiles)
	}
}

func TestConfig_ValidateAuthSkipPattern(t *testing.T) {
	t.Parallel()
	cfg := Config{HTTP: HTTP{Address: "127.0.0.1:8080", RequestTimeout: time.Second}, Health: Health{DatabaseTimeout: time.Second, RedisTimeout: time.Second}, Auth: Auth{SkipHTTPPaths: []string{"/api/v1/[broken"}}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid wildcard error")
	}
}

func TestConfig_ValidateAutoMigration(t *testing.T) {
	t.Parallel()
	cfg := Config{
		HTTP:      HTTP{Address: "127.0.0.1:8080", RequestTimeout: time.Second},
		Health:    Health{DatabaseTimeout: time.Second, RedisTimeout: time.Second},
		Migration: Migration{AutoUp: true, Path: "migrations/postgres"},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want auto migration dependency error")
	}
}

func TestConfig_RejectsNonCanonicalPlatformEventStream(t *testing.T) {
	t.Parallel()
	cfg, err := LoadWithProfile("../../config/config.yaml", "development")
	if err != nil {
		t.Fatal(err)
	}
	cfg.EventBus.Enabled = true
	cfg.EventBus.StreamName = "BILLING_EVENTS"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected non-canonical event stream rejection")
	}
}

func TestLoadUsesSafeOutboxCleanupDefaults(t *testing.T) {
	cfg, err := LoadWithProfile("../../config/config.yaml", "development")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EventBus.PublishedRetention != 7*24*time.Hour || cfg.EventBus.CleanupInterval != time.Hour || cfg.EventBus.CleanupBatchSize != 1000 {
		t.Fatalf("unexpected outbox cleanup defaults: %+v", cfg.EventBus)
	}
}

func TestConfigRejectsOutboxRetentionShorterThanReplayWindow(t *testing.T) {
	cfg, err := LoadWithProfile("../../config/config.yaml", "development")
	if err != nil {
		t.Fatal(err)
	}
	cfg.EventBus.Enabled = true
	cfg.EventBus.PublishedRetention = cfg.EventBus.MaxAge - time.Second
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want outbox retention validation error")
	}
}

func TestConfigRequiresApplicationUpstreamWhenDatabaseEnabled(t *testing.T) {
	t.Parallel()
	cfg, err := LoadWithProfile("../../config/config.yaml", "development")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Database.Enabled = true
	cfg.Database.DSN = "postgres://app:app@127.0.0.1/app?sslmode=disable"
	delete(cfg.Outbound.GRPC, "application")
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want application upstream validation error")
	}
}

func TestConfigOutboundPSKRequiresTLSOrExplicitDevelopmentOptIn(t *testing.T) {
	t.Parallel()
	cfg, err := LoadWithProfile("../../config/config.yaml", "development")
	if err != nil {
		t.Fatal(err)
	}
	application := cfg.Outbound.GRPC["application"]
	application.Auth = ClientAuth{Type: "psk", Token: strings.Repeat("p", 32)}
	cfg.Outbound.GRPC["application"] = application
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "TLS or explicit allow_insecure") {
		t.Fatalf("Validate() error = %v", err)
	}
	application.TLS.AllowInsecure = true
	cfg.Outbound.GRPC["application"] = application
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() with development opt-in error = %v", err)
	}
	if err := validateClientPolicy("application", application.Auth, application.Retry, application.Breaker, application.TLS, true); err == nil || !strings.Contains(err.Error(), "production") {
		t.Fatalf("production validateClientPolicy() error = %v", err)
	}
}

func TestLoadMapsOutboundAllowInsecureEnvironmentOverride(t *testing.T) {
	t.Setenv("APP_OUTBOUND_GRPC_APPLICATION_AUTH_TYPE", "psk")
	t.Setenv("APP_OUTBOUND_GRPC_APPLICATION_AUTH_TOKEN", strings.Repeat("p", 32))
	t.Setenv("APP_OUTBOUND_GRPC_APPLICATION_TLS_ALLOW_INSECURE", "true")

	cfg, err := LoadWithProfile("../../config/config.yaml", "development")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Outbound.GRPC["application"].TLS.AllowInsecure {
		t.Fatal("outbound application allow_insecure environment override was not decoded")
	}
}
