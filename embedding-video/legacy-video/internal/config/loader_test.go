package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMustLoadAppliesSensitiveEnvOverrides(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "video.yml")
	data := []byte(`
Redis:
  Password: "file-redis"
Postgres:
  DSN: "file-postgres"
RustFS:
  AccessKey: "file-ak"
  SecretKey: "file-sk"
asr:
  api-key: "file-asr"
embedding:
  api-key: "file-embedding"
`)
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("POSTGRES_DSN", "env-postgres")
	t.Setenv("REDIS_PASSWORD", "env-redis")
	t.Setenv("COS_SECRET_ID", "env-ak")
	t.Setenv("COS_SECRET_KEY", "env-sk")
	t.Setenv("ASR_API_KEY", "env-asr")
	t.Setenv("EMBEDDING_API_KEY", "env-embedding")

	cfg := MustLoad(cfgPath)

	if cfg.Postgres.DSN != "env-postgres" {
		t.Fatalf("Postgres.DSN = %q, want env override", cfg.Postgres.DSN)
	}
	if cfg.Redis.Password != "env-redis" {
		t.Fatalf("Redis.Password = %q, want env override", cfg.Redis.Password)
	}
	if cfg.RustFS.AccessKey != "env-ak" || cfg.RustFS.SecretKey != "env-sk" {
		t.Fatalf("RustFS credentials = %q/%q, want env overrides", cfg.RustFS.AccessKey, cfg.RustFS.SecretKey)
	}
	if cfg.ASR.APIKey != "env-asr" {
		t.Fatalf("ASR.APIKey = %q, want env override", cfg.ASR.APIKey)
	}
	if cfg.Embedding.APIKey != "env-embedding" {
		t.Fatalf("Embedding.APIKey = %q, want env override", cfg.Embedding.APIKey)
	}
}

func TestMustLoadAppliesDotEnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "video.yml")
	data := []byte(`
Postgres:
  DSN: ""
RustFS:
  AccessKey: ""
  SecretKey: ""
asr:
  api-key: ""
embedding:
  api-key: ""
`)
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("VIDEO_APP_ENV_FILE=.env.local\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".env.local"), []byte(`
POSTGRES_DSN=dotenv-postgres
COS_SECRET_ID=dotenv-ak
COS_SECRET_KEY=dotenv-sk
DASHSCOPE_API_KEY=dotenv-ai
`), 0o600); err != nil {
		t.Fatalf("write .env.local: %v", err)
	}

	cleanupEnv := cleanEnv(t, "VIDEO_APP_ENV_FILE", "POSTGRES_DSN", "COS_SECRET_ID", "COS_SECRET_KEY", "DASHSCOPE_API_KEY")
	defer cleanupEnv()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	cfg := MustLoad(cfgPath)

	if cfg.Postgres.DSN != "dotenv-postgres" {
		t.Fatalf("Postgres.DSN = %q, want dotenv override", cfg.Postgres.DSN)
	}
	if cfg.RustFS.AccessKey != "dotenv-ak" || cfg.RustFS.SecretKey != "dotenv-sk" {
		t.Fatalf("RustFS credentials = %q/%q, want dotenv overrides", cfg.RustFS.AccessKey, cfg.RustFS.SecretKey)
	}
	if cfg.ASR.APIKey != "dotenv-ai" || cfg.Embedding.APIKey != "dotenv-ai" {
		t.Fatalf("AI credentials = %q/%q, want dotenv override", cfg.ASR.APIKey, cfg.Embedding.APIKey)
	}
}

func cleanEnv(t *testing.T, names ...string) func() {
	t.Helper()
	originals := make(map[string]string, len(names))
	present := make(map[string]bool, len(names))
	for _, name := range names {
		value, ok := os.LookupEnv(name)
		originals[name] = value
		present[name] = ok
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("unset %s: %v", name, err)
		}
	}
	return func() {
		for _, name := range names {
			if present[name] {
				_ = os.Setenv(name, originals[name])
			} else {
				_ = os.Unsetenv(name)
			}
		}
	}
}
