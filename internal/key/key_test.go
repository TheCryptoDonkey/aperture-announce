package key

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateKey(t *testing.T) {
	k := Generate()
	if len(k) != 64 {
		t.Errorf("key length = %d, want 64", len(k))
	}
	for _, c := range k {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("key contains non-hex char: %c", c)
			break
		}
	}
}

func TestPersistAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "announce.key")

	k := Generate()
	if err := Persist(k, path); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != k {
		t.Errorf("loaded key = %q, want %q", loaded, k)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file permissions = %o, want 600", perm)
	}
}

func TestLoadNonExistent(t *testing.T) {
	_, err := Load("/nonexistent/path/key")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestResolve_ExplicitKey(t *testing.T) {
	k := Generate()
	resolved, err := Resolve(k, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if resolved != k {
		t.Error("explicit key should be returned as-is")
	}
}

func TestResolve_AutoGenerate(t *testing.T) {
	dir := t.TempDir()
	k1, err := Resolve("", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(k1) != 64 {
		t.Error("auto-generated key should be 64 chars")
	}

	k2, err := Resolve("", dir)
	if err != nil {
		t.Fatal(err)
	}
	if k1 != k2 {
		t.Error("second call should return persisted key")
	}
}
