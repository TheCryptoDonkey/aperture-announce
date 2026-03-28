# GEMINI.md — aperture-announce

Standalone Go binary that reads Aperture's YAML config and publishes kind 31402 Nostr events for decentralised L402 service discovery.

## Commands

```bash
go build ./cmd/aperture-announce   # Build
go test ./...                      # Run all tests
go vet ./...                       # Lint
```

All three must pass before committing.

## Project structure

```
cmd/aperture-announce/main.go      # CLI entry point, flag parsing
internal/
  config/config.go                 # Aperture YAML parser
  announce/announce.go             # Event building (kind 31402)
  announce/publish.go              # Relay publishing
  key/key.go                       # Key generation + persistence
  validate/validate.go             # URL validation (private host rejection)
testdata/
  sample-conf.yaml                 # Example Aperture config for tests
```

All application code is in `internal/` — this is a binary, not a library.

## Conventions

- **British English** — colour, initialise, behaviour, licence
- **Go standard layout** — `cmd/` for binaries, `internal/` for private packages
- **Commit messages** — `type: description` format (e.g. `feat:`, `fix:`, `docs:`)
- **No Co-Authored-By lines** in commits

## Event format (critical)

The kind 31402 event must match [402-announce](https://github.com/forgesworn/402-announce) output:

- `pmi` tags for payment method (not `payment`)
- `price` tags with positional elements: `["price", "capability", "amount", "currency"]` (not JSON)
- Capabilities in the `content` field as JSON: `{"capabilities": [...]}`

## Testing

Tests use Go's standard `testing` package with table-driven patterns. Each package in `internal/` has a corresponding `_test.go` file.
