package app

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type serverJSONInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Value       string `json:"value"`
	Type        string `json:"type"`
	IsRequired  bool   `json:"isRequired"`
	IsSecret    bool   `json:"isSecret"`
}

type serverJSON struct {
	Schema      string `json:"$schema"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Packages    []struct {
		RegistryType string `json:"registryType"`
		Identifier   string `json:"identifier"`
		RuntimeHint  string `json:"runtimeHint"`
		Transport    struct {
			Type string `json:"type"`
		} `json:"transport"`
		PackageArguments []serverJSONInput `json:"packageArguments"`
		RuntimeArguments []struct {
			serverJSONInput
			Variables map[string]json.RawMessage `json:"variables"`
		} `json:"runtimeArguments"`
		EnvironmentVariables []serverJSONInput `json:"environmentVariables"`
	} `json:"packages"`
}

// TestServerJSONMatchesRegistryRules checks server.json against the MCP Registry
// schema 2025-12-11 rules that the schema itself can express offline, and the
// OCI ownership label in .goreleaser.yaml (checked by the registry at publish).
func TestServerJSONMatchesRegistryRules(t *testing.T) {
	raw, err := os.ReadFile("../../server.json")
	require.NoError(t, err)
	var s serverJSON
	require.NoError(t, json.Unmarshal(raw, &s))

	assert.Equal(t, "https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json", s.Schema)
	assert.Regexp(t, `^[a-zA-Z0-9.-]+/[a-zA-Z0-9._-]+$`, s.Name)
	assert.Equal(t, 1, strings.Count(s.Name, "/"))
	assert.LessOrEqual(t, len(s.Name), 200)
	assert.GreaterOrEqual(t, len(s.Name), 3)
	assert.NotEmpty(t, s.Description)
	assert.LessOrEqual(t, len(s.Description), 100)
	assert.NotEmpty(t, s.Title)
	assert.LessOrEqual(t, len(s.Title), 100)
	// a concrete version, no range or "latest"
	assert.Regexp(t, `^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`, s.Version)

	require.Len(t, s.Packages, 1)
	p := s.Packages[0]
	assert.Equal(t, "oci", p.RegistryType)
	assert.Equal(t, "ghcr.io/werbot/werbot:v"+s.Version, p.Identifier)
	assert.Equal(t, "stdio", p.Transport.Type)
	require.Len(t, p.PackageArguments, 1)
	assert.Equal(t, "positional", p.PackageArguments[0].Type)
	assert.Equal(t, "mcp", p.PackageArguments[0].Value)

	// runtimeHint is required with runtimeArguments; every {placeholder} is declared
	require.NotEmpty(t, p.RuntimeArguments)
	assert.NotEmpty(t, p.RuntimeHint)
	placeholder := regexp.MustCompile(`\{([a-z_]+)\}`)
	for _, a := range p.RuntimeArguments {
		for _, m := range placeholder.FindAllStringSubmatch(a.Value, -1) {
			assert.Contains(t, a.Variables, m[1])
		}
	}

	// the server address is required; the profile mount carries the login, so no secret input exists
	env := map[string]serverJSONInput{}
	for _, e := range p.EnvironmentVariables {
		env[e.Name] = e
		assert.False(t, e.IsSecret, e.Name)
	}
	assert.True(t, env["WERBOT_SERVER"].IsRequired)
	assert.NotContains(t, string(raw), `"isSecret": true`)

	// the registry verifies image ownership by this label (goreleaser publishes the image)
	gr, err := os.ReadFile("../../.goreleaser.yaml")
	require.NoError(t, err)
	assert.Regexp(t, `(?m)^\s+io\.modelcontextprotocol\.server\.name: `+regexp.QuoteMeta(s.Name)+`\s*$`, string(gr))

	readme, err := os.ReadFile("../../README.md")
	require.NoError(t, err)
	assert.Contains(t, string(readme), "server.json")
}
