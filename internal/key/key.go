package key

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Generate creates a new 32-byte random secret key as 64-char hex.
func Generate() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}

// Persist writes a key to a file with 0600 permissions.
// Creates parent directories with 0700 if needed.
func Persist(key, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create key directory: %w", err)
	}
	return os.WriteFile(path, []byte(key+"\n"), 0o600)
}

// Load reads a key from a file.
func Load(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// Resolve returns the key to use: explicit if provided, otherwise
// auto-generates and persists to keyDir/announce.key.
func Resolve(explicit, keyDir string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}

	path := filepath.Join(keyDir, "announce.key")

	// Try loading existing
	if k, err := Load(path); err == nil {
		return k, nil
	}

	// Generate and persist
	k := Generate()
	if err := Persist(k, path); err != nil {
		return "", fmt.Errorf("persist auto-generated key: %w", err)
	}
	return k, nil
}
