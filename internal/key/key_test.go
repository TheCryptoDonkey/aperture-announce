package key

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateKey(t *testing.T) {
	k, err := Generate()
	require.NoError(t, err)
	require.Len(t, k, 64)
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

	k, err := Generate()
	require.NoError(t, err)
	require.NoError(t, Persist(k, path))

	loaded, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, k, loaded)

	info, err := os.Stat(path)
	require.NoError(t, err)
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file permissions = %o, want 600", perm)
	}
}

func TestLoadNonExistent(t *testing.T) {
	_, err := Load("/nonexistent/path/key")
	require.Error(t, err)
}

func TestResolve_ExplicitKey(t *testing.T) {
	k, err := Generate()
	require.NoError(t, err)
	resolved, err := Resolve(k, t.TempDir())
	require.NoError(t, err)
	require.Equal(t, k, resolved)
}

func TestResolve_AutoGenerate(t *testing.T) {
	dir := t.TempDir()
	k1, err := Resolve("", dir)
	require.NoError(t, err)
	require.Len(t, k1, 64)

	k2, err := Resolve("", dir)
	require.NoError(t, err)
	require.Equal(t, k1, k2)
}

func TestValidate(t *testing.T) {
	k, err := Generate()
	require.NoError(t, err)
	require.NoError(t, Validate(k))

	// Uppercase hex should be accepted
	upper := "AABBCCDDAABBCCDDAABBCCDDAABBCCDDAABBCCDDAABBCCDDAABBCCDDAABBCCDD"
	require.NoError(t, Validate(upper))

	invalid := []struct {
		name string
		key  string
	}{
		{"too short", "abcd"},
		{"too long", "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddee"},
		{"non-hex chars", "gghhiijjgghhiijjgghhiijjgghhiijjgghhiijjgghhiijjgghhiijjgghhiijj"},
		{"empty", ""},
	}
	for _, tc := range invalid {
		if err := Validate(tc.key); err == nil {
			t.Errorf("Validate(%q) [%s]: expected error", tc.key, tc.name)
		}
	}
}

func TestValidate_DegenerateKey(t *testing.T) {
	cases := []struct {
		name string
		key  string
	}{
		{"all zeros", "0000000000000000000000000000000000000000000000000000000000000000"},
		{"all f", "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"},
		{"all a", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	for _, tc := range cases {
		if err := Validate(tc.key); err == nil {
			t.Errorf("Validate(%s): expected error for degenerate key", tc.name)
		}
	}
}

func TestResolve_UppercaseKeyNormalised(t *testing.T) {
	upper := "AABBCCDDAABBCCDDAABBCCDDAABBCCDDAABBCCDDAABBCCDDAABBCCDDAABBCCDD"
	resolved, err := Resolve(upper, t.TempDir())
	require.NoError(t, err)
	expected := "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd"
	require.Equal(t, expected, resolved)
}

func TestResolve_InvalidExplicitKey(t *testing.T) {
	_, err := Resolve("not-a-valid-key", t.TempDir())
	require.Error(t, err)
}

func TestResolve_CorruptedKeyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "announce.key")
	require.NoError(t, os.WriteFile(path, []byte("corrupted\n"), 0o600))
	_, err := Resolve("", dir)
	require.Error(t, err)
}

func TestPersist_RejectsInvalidKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "announce.key")
	require.Error(t, Persist("not-valid", path))
	require.Error(t, Persist("", path))
}

func TestResolve_RaceProtection(t *testing.T) {
	// Simulate the race: pre-create the key file between Generate and Persist.
	// Resolve should detect the existing file and load it instead of failing.
	dir := t.TempDir()
	existing, err := Generate()
	require.NoError(t, err)
	path := filepath.Join(dir, "announce.key")
	require.NoError(t, os.WriteFile(path, []byte(existing+"\n"), 0o600))

	// Resolve with no explicit key should load the existing file.
	resolved, err := Resolve("", dir)
	require.NoError(t, err)
	require.Equal(t, existing, resolved)
}
