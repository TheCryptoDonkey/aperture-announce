# Event Format Improvements — Design Spec

Improve kind 31402 event output to match 402-announce conventions, handle Aperture config fields we currently ignore, and pre-empt upstream feedback.

## Context

aperture-announce reads Aperture's YAML config and publishes kind 31402 Nostr events. The canonical event format is defined by 402-announce (TypeScript). Comparing our output against 402-announce and the upstream Aperture Service struct reveals six gaps that would undermine credibility with the Aperture team.

## Changes

### 1. Strip regex from endpoint paths

**Problem:** Capability endpoints contain Aperture's path regex (e.g. `/v1/loop/.*`) instead of usable API paths. An agent consuming this via 402-mcp would try `GET /v1/loop/.*` literally.

**Solution:** Add `CleanEndpoint(regex string) string` in `internal/announce/announce.go`. Operations applied in order:

1. Strip anchors: remove leading `^` and trailing `$`
2. Find first occurrence of `.*`, `.+`, `[`, `(`, `?`, or `{` — truncate there
3. Trim trailing `/` from truncation artifacts (but preserve a single trailing `/` on path segments)
4. If result is just `/` or empty, return empty string (omitted via `omitempty`)

Other regex constructs (`?`, `{n,m}`) are handled by step 2 — they trigger truncation at their position. This covers all practical Aperture path patterns without needing a full regex parser.

| Input | Output |
|-------|--------|
| `^/v1/loop/.*$` | `/v1/loop/` |
| `/v1/pool/.*` | `/v1/pool/` |
| `^/looprpc.SwapServer/LoopOutTerms.*$` | `/looprpc.SwapServer/LoopOutTerms` |
| `^/v1/(quote\|swap)/.*$` | `/v1/` |
| `^/.*$` | `` (omitted) |
| `.*` | `` (omitted) |
| `/v1/items?` | `/v1/item` |

### 2. Handle dynamic pricing honestly

**Problem:** Services with dynamic pricing and no static fallback emit `["price", "cap", "0", "sats"]` — agents see "free" when the price is actually determined at request time.

**Solution:** Two cases:

**Dynamic + no static fallback (price == 0):**
- Skip the `price` tag for capabilities on this service
- Add `["t", "dynamic-pricing"]` topic tag to the event
- Set `"pricing": "dynamic"` on each capability in the content JSON

**Dynamic + static fallback (price > 0):**
- Keep the `price` tag (the static price is a reasonable estimate)
- Set `"pricing": "dynamic"` on each capability in the content JSON
- Add `["t", "dynamic-pricing"]` topic tag to the event

When `pricing` is absent from a capability, agents assume the price tag is definitive.

The `dynamic-pricing` topic tag is a coarse event-level signal. Agents should check per-capability `pricing` fields for specifics — the topic tag just indicates that at least one capability in the event uses dynamic pricing.

**Interaction with zero-price default:** The existing logic defaults `price: 0` to 1 sat when dynamic pricing is off (matching Aperture's `defaultServicePrice`). This default does NOT apply when `DynamicPrice` is true — a dynamic-priced service with no static fallback stays at 0 and its price tag is omitted. The boundary: `price == 0 && !DynamicPrice` triggers the default; `price == 0 && DynamicPrice` triggers price tag omission.

**Interaction with auth: "off":** A service with `auth: "off"` and `dynamicprice.enabled: true` is valid (free discovery, dynamic pricing for premium use). Both changes apply independently: price tag is omitted (dynamic + no fallback), auth shows `"none"` in content.

### 3. Separate name and about tags

**Problem:** Both tags contain the identical auto-generated string. 402-announce uses `name` as a short label and `about` as a longer description.

**Solution:**
- `name` tag: service names joined — e.g. `"loop-rpc, pool-rpc"` (single service: just `"loop-rpc"`). If more than 5 services, truncate to first 3 names plus `"and N more"` to keep the tag scannable.
- `about` tag: full description — e.g. `"L402-gated API via Aperture — loop-rpc, pool-rpc"` (always includes all service names regardless of count).

### 4. Parse auth level from Aperture config

**Problem:** Aperture supports `auth: "off"`, `auth: "on"`, and `auth: "freebie N"`. Agents benefit from knowing whether a service requires payment upfront.

**Solution:** Add `Auth string` to the `Service` struct in `internal/config/config.go`. Parse the `auth` field from YAML. Represent it per-capability in the content JSON (not as an event-level tag, since auth is per-service in Aperture and services can differ).

Content JSON field values:
- `auth: "on"` or empty (default) → omit field (agents assume payment required)
- `auth: "off"` or `auth: "false"` → `"auth": "none"`
- `auth: "freebie 5"` → `"auth": "freebie 5"` (preserved as-is)

All capabilities from the same Aperture service share the same auth level.

**Validation:** The config parser should warn (not fatal) on unrecognised auth values. Accepted values: empty, `"on"`, `"true"`, `"off"`, `"false"`, or matching `"freebie N"` where N is a positive integer. Unrecognised values are treated as `"on"` (the safe default) with a stderr warning.

### 5. Parse timeout from Aperture config

**Problem:** Aperture's `Timeout` field controls L402 token validity. An agent paying for access should know if it expires.

**Solution:** Add `Timeout int64` to the `Service` struct. Parse the `timeout` field from YAML. Represent it per-capability in the content JSON as `"timeout": N` (seconds). Omit when 0 — in Aperture, `timeout: 0` (or unset) means no explicit expiry on the L402 token.

All capabilities from the same Aperture service share the same timeout.

### 6. Custom topics via --topics flag

**Problem:** Hardcoded topics (`l402`, `api`, `aperture`) cannot be customised.

**Solution:** Add `--topics` CLI flag (comma-separated) and `ANNOUNCE_TOPICS` env var fallback. Default topics (`l402`, `api`, `aperture`) are always included first. User topics are appended. Deduplication preserves first occurrence. Cap at 50 total (matching 402-announce's limit). The `dynamic-pricing` topic from Change 2 counts toward this cap.

## BuildEvent signature

The current signature `BuildEvent(secretKey string, cfg *config.ApertureConfig, publicURL string, picture string)` has too many positional parameters and needs to accept topics. Replace with an options struct:

```go
// BuildOptions holds optional parameters for event construction.
type BuildOptions struct {
    PublicURL string
    Picture   string
    Topics    []string
}

func BuildEvent(secretKey string, cfg *config.ApertureConfig, opts BuildOptions) (*nostr.Event, error)
```

This is a breaking change to the function signature but the only caller is `main.go`, so the migration is trivial.

## Updated capability struct

```go
type capability struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Endpoint    string `json:"endpoint,omitempty"`
    Pricing     string `json:"pricing,omitempty"`
    Auth        string `json:"auth,omitempty"`
    Timeout     int64  `json:"timeout,omitempty"`
}
```

## Updated config Service struct

```go
type Service struct {
    Name         string
    HostRegexp   string
    PathRegexp   string
    Price        int64
    DynamicPrice bool
    Capabilities []string
    Auth         string
    Timeout      int64
}
```

With corresponding raw struct additions:

```go
type rawService struct {
    // ... existing fields ...
    Auth    string `yaml:"auth"`
    Timeout int64  `yaml:"timeout"`
}
```

## Example event output

Given this Aperture config:

```yaml
services:
  - name: "loop-rpc"
    hostregexp: "api.example.com"
    pathregexp: "^/v1/loop/.*$"
    price: 500
    capabilities: "quote,swap"
    auth: "freebie 3"
    timeout: 3600

  - name: "pool-rpc"
    hostregexp: "api.example.com"
    pathregexp: "^/v1/pool/.*$"
    dynamicprice:
      enabled: true
      grpcaddress: "localhost:10010"

  - name: "status"
    hostregexp: "api.example.com"
    pathregexp: "^/v1/status$"
    auth: "off"
    price: 0
```

The event would be:

```json
{
  "kind": 31402,
  "tags": [
    ["d", "aperture-api.example.com"],
    ["name", "loop-rpc, pool-rpc, status"],
    ["url", "https://api.example.com"],
    ["about", "L402-gated API via Aperture — loop-rpc, pool-rpc, status"],
    ["pmi", "bitcoin-lightning-bolt11"],
    ["t", "l402"],
    ["t", "api"],
    ["t", "aperture"],
    ["t", "dynamic-pricing"],
    ["price", "quote", "500", "sats"],
    ["price", "swap", "500", "sats"],
    ["price", "status", "1", "sats"]
  ],
  "content": "{\"capabilities\":[{\"name\":\"quote\",\"description\":\"quote via loop-rpc\",\"endpoint\":\"/v1/loop/\",\"auth\":\"freebie 3\",\"timeout\":3600},{\"name\":\"swap\",\"description\":\"swap via loop-rpc\",\"endpoint\":\"/v1/loop/\",\"auth\":\"freebie 3\",\"timeout\":3600},{\"name\":\"pool-rpc\",\"description\":\"Access pool-rpc\",\"endpoint\":\"/v1/pool/\",\"pricing\":\"dynamic\"},{\"name\":\"status\",\"description\":\"Access status\",\"endpoint\":\"/v1/status\",\"auth\":\"none\"}]}"
}
```

Note:
- `pool-rpc` has no price tag (dynamic pricing enabled, no static fallback → price tag omitted) but IS in capabilities with `"pricing": "dynamic"`
- `status` has `price: 0` in YAML but gets 1 sat in the event — this is the existing zero-price default (`DefaultServicePrice = 1`) which applies because `status` has no dynamic pricing. Auth is `"none"` — the price structure exists but is currently unenforced.
- `quote`/`swap` have `auth: "freebie 3"` and `timeout: 3600`
- Endpoints are cleaned: `^/v1/loop/.*$` → `/v1/loop/`, `^/v1/status$` → `/v1/status`
- `dynamic-pricing` topic added because pool-rpc uses dynamic pricing

## Files changed

| File | Changes |
|------|---------|
| `internal/config/config.go` | Add `Auth`, `Timeout` to Service struct and raw parsing |
| `internal/config/config_test.go` | Tests for auth, timeout, zero-price-with-auth-off |
| `internal/announce/announce.go` | `CleanEndpoint()`, name/about split, dynamic pricing logic, auth/timeout/pricing in content, custom topics |
| `internal/announce/announce_test.go` | Tests for all event building changes |
| `cmd/aperture-announce/main.go` | `--topics` flag, `ANNOUNCE_TOPICS` env var |
| `cmd/aperture-announce/main_test.go` | CLI tests for topics flag |
| `testdata/sample-conf.yaml` | Add auth, timeout, dynamic pricing service |
| `docs/event-format.md` | Document new tags and content fields |
| `schemas/kind-31402.schema.json` | Add auth, timeout, pricing definitions |
| `README.md` | Update usage section for --topics |
| `llms.txt` | Update key concepts |
| `llms-full.txt` | Full update with new format |

## Backward compatibility

These changes alter the event structure. Since kind 31402 is a replaceable event (NIP-33), the new event replaces the old on relays — there is no period where both formats coexist.

**Breaking changes for consumers:**
- `name` tag value changes from the full description to a short label
- `price` tags may be absent for dynamic-priced services (previously showed `0`)
- `endpoint` values change from regex patterns to cleaned base paths

**Additive (non-breaking):**
- New content JSON fields: `pricing`, `auth`, `timeout`
- New topic tags: `dynamic-pricing`, custom user topics
- `about` tag now differs from `name` (previously identical)

This is acceptable: the tool is at v0.1.0, there are no known third-party consumers beyond 402-mcp (which searches by kind and `url` tag, not `name`), and the changes fix genuinely broken behaviour (regex endpoints, misleading prices).

## Known limitations (document, do not fix)

- **authwhitelistpaths** — Aperture supports per-path auth exemptions within a service. We represent auth at the service level. Path-level granularity would require splitting capabilities in ways that don't map cleanly to Aperture's config format.
- **Per-capability pricing** — Aperture's gRPC pricer can charge differently per capability. We announce the service-level static price for all capabilities within a service.
- **Content size** — 402-announce caps content at 64KB. Typical Aperture configs are nowhere near this limit.
- **Single payment method** — We hardcode `bitcoin-lightning-bolt11` since Aperture only supports Lightning. Multi-method support (Cashu, etc.) is out of scope.
