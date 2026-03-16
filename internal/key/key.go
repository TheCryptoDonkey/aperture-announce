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
func Generate() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand failed: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Validate checks that a key is a 64-character hex string (case-insensitive).
func Validate(k string) error {
	if len(k) != 64 {
		return fmt.Errorf("key must be 64 hex characters, got %d", len(k))
	}
	for _, c := range k {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return fmt.Errorf("key contains invalid character: %c", c)
		}
	}
	return nil
}

// Normalise lowercases a hex key for consistent internal use.
func Normalise(k string) string {
	return strings.ToLower(k)
}

// Persist writes a key to a file with 0600 permissions.
// Creates parent directories with 0700 if needed.
func Persist(key, path string) error {
	if err := Validate(key); err != nil {
		return fmt.Errorf("refusing to persist invalid key: %w", err)
	}
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
		if err := Validate(explicit); err != nil {
			return "", fmt.Errorf("invalid explicit key: %w", err)
		}
		return Normalise(explicit), nil
	}

	path := filepath.Join(keyDir, "announce.key")

	// Try loading existing
	if k, err := Load(path); err == nil {
		if err := Validate(k); err != nil {
			return "", fmt.Errorf("corrupted key file %s: %w", path, err)
		}
		return Normalise(k), nil
	}

	// Generate new key
	k, err := Generate()
	if err != nil {
		return "", err
	}

	// Atomically create the file with O_EXCL to prevent TOCTOU races.
	// If two processes race here, only one will succeed; the loser
	// retries by loading the winner's key.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create key directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			// Another process created the file first — load their key.
			k2, loadErr := Load(path)
			if loadErr != nil {
				return "", fmt.Errorf("load racing key file: %w", loadErr)
			}
			if err := Validate(k2); err != nil {
				return "", fmt.Errorf("corrupted key file %s: %w", path, err)
			}
			return Normalise(k2), nil
		}
		return "", fmt.Errorf("create key file: %w", err)
	}
	if _, err := f.WriteString(k + "\n"); err != nil {
		f.Close()
		return "", fmt.Errorf("write key file: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close key file: %w", err)
	}

	return k, nil
}
