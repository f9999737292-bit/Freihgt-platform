package storage

import (
	"os"
	"strings"
	"testing"
)

func TestS3ConfigFailClosed(t *testing.T) {
	t.Setenv("EDO_OBJECT_STORAGE_PROVIDER", "s3")
	t.Setenv("EDO_OBJECT_STORAGE_ENDPOINT", "")
	t.Setenv("EDO_OBJECT_STORAGE_REGION", "ru-1")
	t.Setenv("EDO_OBJECT_STORAGE_BUCKET", "edo-attachments")
	t.Setenv("EDO_OBJECT_STORAGE_ACCESS_KEY", "test-access")
	t.Setenv("EDO_OBJECT_STORAGE_SECRET_KEY", "super-secret-value")
	_, err := LoadConfig(t.TempDir())
	if err == nil || strings.Contains(err.Error(), "super-secret-value") || strings.Contains(err.Error(), "test-access") {
		t.Fatalf("config error leaked or was accepted: %v", err)
	}
	store, openErr := Open(Config{Provider: "s3"})
	if openErr == nil || store != nil {
		t.Fatal("invalid s3 config opened a store")
	}
}

func TestS3ConfigRejectsUnsafeEndpoints(t *testing.T) {
	base := map[string]string{
		"EDO_OBJECT_STORAGE_PROVIDER":   "s3",
		"EDO_OBJECT_STORAGE_REGION":     "ru-1",
		"EDO_OBJECT_STORAGE_BUCKET":     "edo-attachments",
		"EDO_OBJECT_STORAGE_ACCESS_KEY": "test-access",
		"EDO_OBJECT_STORAGE_SECRET_KEY": "test-secret",
		"EDO_OBJECT_STORAGE_TLS":        "true",
	}
	for _, endpoint := range []string{
		"file:///tmp/bucket",
		"http://127.0.0.1:9000",
		"https://169.254.169.254",
		"https://user:pass@storage.example",
		"https://10.0.0.5",
	} {
		for key, value := range base {
			t.Setenv(key, value)
		}
		t.Setenv("EDO_OBJECT_STORAGE_ENDPOINT", endpoint)
		if _, err := LoadConfig(t.TempDir()); err == nil {
			t.Fatalf("accepted endpoint %s", endpoint)
		}
	}
}

func TestLocalConfigDoesNotRequireS3Secrets(t *testing.T) {
	t.Setenv("EDO_OBJECT_STORAGE_PROVIDER", "")
	t.Setenv("EDO_OBJECT_STORAGE_SECRET_KEY", "")
	root := t.TempDir()
	cfg, err := LoadConfig(root)
	if err != nil || cfg.Provider != "local" {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
	store, err := Open(cfg)
	if err != nil || store == nil {
		t.Fatal(err)
	}
}

func TestExplicitS3DoesNotFallBackWhenUnset(t *testing.T) {
	os.Unsetenv("EDO_OBJECT_STORAGE_ENDPOINT")
	t.Setenv("EDO_OBJECT_STORAGE_PROVIDER", "s3")
	t.Setenv("EDO_OBJECT_STORAGE_ENDPOINT", "https://s3.example")
	t.Setenv("EDO_OBJECT_STORAGE_REGION", "")
	t.Setenv("EDO_OBJECT_STORAGE_BUCKET", "edo-attachments")
	t.Setenv("EDO_OBJECT_STORAGE_ACCESS_KEY", "test-access")
	t.Setenv("EDO_OBJECT_STORAGE_SECRET_KEY", "test-secret")
	if _, err := LoadConfig(t.TempDir()); err == nil {
		t.Fatal("missing region was accepted")
	}
}
