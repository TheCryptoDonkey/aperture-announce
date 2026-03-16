package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSingleService(t *testing.T) {
	yml := `
services:
  - name: "my-api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
`
	cfg, err := Parse([]byte(yml))
	require.NoError(t, err)
	require.Len(t, cfg.Services, 1)
	s := cfg.Services[0]
	require.Equal(t, "my-api", s.Name)
	require.Equal(t, int64(100), s.Price)
	require.Equal(t, "/v1/.*", s.PathRegexp)
}

func TestParseMultipleServices(t *testing.T) {
	yml := `
services:
  - name: "read-api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/read"
    price: 50
  - name: "write-api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/write"
    price: 200
`
	cfg, err := Parse([]byte(yml))
	require.NoError(t, err)
	require.Len(t, cfg.Services, 2)
}

func TestParseCapabilities(t *testing.T) {
	yml := `
services:
  - name: "my-api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
    capabilities: "read,write,admin"
`
	cfg, err := Parse([]byte(yml))
	require.NoError(t, err)
	caps := cfg.Services[0].Capabilities
	require.Len(t, caps, 3)
	require.Equal(t, "read", caps[0])
	require.Equal(t, "write", caps[1])
	require.Equal(t, "admin", caps[2])
}

func TestParseDynamicPricing(t *testing.T) {
	yml := `
services:
  - name: "dynamic-api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    dynamicprice:
      enabled: true
      grpcaddress: "localhost:10010"
`
	cfg, err := Parse([]byte(yml))
	require.NoError(t, err)
	s := cfg.Services[0]
	require.True(t, s.DynamicPrice)
	require.Equal(t, int64(0), s.Price)
}

func TestParseNoServices(t *testing.T) {
	yml := `
listenaddr: "localhost:8081"
`
	_, err := Parse([]byte(yml))
	require.Error(t, err)
}

func TestParseEmptyServices(t *testing.T) {
	yml := `
services: []
`
	_, err := Parse([]byte(yml))
	require.Error(t, err)
}

func TestParseEmptyServiceName(t *testing.T) {
	yml := `
services:
  - name: ""
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
`
	_, err := Parse([]byte(yml))
	require.Error(t, err)
}

func TestParseNegativePrice(t *testing.T) {
	yml := `
services:
  - name: "my-api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: -100
`
	_, err := Parse([]byte(yml))
	require.Error(t, err)
}

func TestParseZeroPriceDefaultsToOne(t *testing.T) {
	yml := `
services:
  - name: "my-api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
`
	cfg, err := Parse([]byte(yml))
	require.NoError(t, err)
	s := cfg.Services[0]
	require.Equal(t, DefaultServicePrice, s.Price)
}

func TestParseZeroPriceWithDynamicPricingStaysZero(t *testing.T) {
	yml := `
services:
  - name: "dynamic-api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    dynamicprice:
      enabled: true
      grpcaddress: "localhost:10010"
`
	cfg, err := Parse([]byte(yml))
	require.NoError(t, err)
	s := cfg.Services[0]
	require.Equal(t, int64(0), s.Price)
}

func TestParseAuth(t *testing.T) {
	yml := `
services:
  - name: "api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
    auth: "freebie 5"
`
	cfg, err := Parse([]byte(yml))
	require.NoError(t, err)
	require.Equal(t, "freebie 5", cfg.Services[0].Auth)
}

func TestParseAuthOff(t *testing.T) {
	yml := `
services:
  - name: "api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
    auth: "off"
`
	cfg, err := Parse([]byte(yml))
	require.NoError(t, err)
	require.Equal(t, "off", cfg.Services[0].Auth)
}

func TestParseAuthDefault(t *testing.T) {
	yml := `
services:
  - name: "api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
`
	cfg, err := Parse([]byte(yml))
	require.NoError(t, err)
	require.Equal(t, "", cfg.Services[0].Auth)
}

func TestParseTimeout(t *testing.T) {
	yml := `
services:
  - name: "api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
    timeout: 3600
`
	cfg, err := Parse([]byte(yml))
	require.NoError(t, err)
	require.Equal(t, int64(3600), cfg.Services[0].Timeout)
}

func TestParseNegativeTimeout(t *testing.T) {
	yml := `
services:
  - name: "api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
    timeout: -1
`
	_, err := Parse([]byte(yml))
	require.Error(t, err)
}

func TestParseTooManyServices(t *testing.T) {
	// Build YAML with 1001 services
	yml := "services:\n"
	for i := 0; i < 1001; i++ {
		yml += "  - name: \"svc-" + fmt.Sprintf("%d", i) + "\"\n"
		yml += "    hostregexp: \"api.example.com\"\n"
		yml += "    pathregexp: \"/v1/.*\"\n"
		yml += "    price: 1\n"
	}
	_, err := Parse([]byte(yml))
	require.Error(t, err)
}

func TestParseExcessivePrice(t *testing.T) {
	data := []byte(`services:
  - name: "expensive"
    hostregexp: "example.com"
    pathregexp: "/v1/.*"
    price: 99999999999999
`)
	_, err := Parse(data)
	require.Error(t, err)
}

func TestParseAuthUnrecognisedWarns(t *testing.T) {
	yml := `
services:
  - name: "api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
    auth: "maybe"
`
	cfg, err := Parse([]byte(yml))
	require.NoError(t, err)
	// Unrecognised auth values are stored as-is. Warning emitted by caller.
	require.Equal(t, "maybe", cfg.Services[0].Auth)
}
