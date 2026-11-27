package app

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// When the page count changes, update README.md, site/astro.config.mjs and this list.
var docPages = []string{
	"agents", "cli", "configuration", "connect", "connectors", "databases", "failure-modes", "gateway", "install", "jit", "mfa",
	"migrate-from-boundary", "policies", "scaling", "security", "ssh-hardening", "sso", "upgrading", "vnet",
	"web-terminal", "web",
}

// Anchors that plans, parity rows and other pages link to; they never change.
var stableAnchors = map[string][]string{
	"jit.md":                   {"access-requests", "approval-policies", "break-glass", "locks"},
	"policies.md":              {"classes", "command-rules", "custom-cedar", "command-approvals", "output-masking"},
	"agents.md":                {"werbot-mcp", "limits", "kill-switch", "run-timeline"},
	"mfa.md":                   {"ssh", "fresh-mfa-for-admin-actions"},
	"sso.md":                   {"set-up-an-idp", "claims-mapping", "idp-unavailable"},
	"cli.md":                   {"log-in", "commands"},
	"web-terminal.md":          {"live-view"},
	"migrate-from-boundary.md": {"mapping"},
}

var (
	mdLink      = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	nonSlugChar = regexp.MustCompile(`[^a-z0-9 _-]`)
)

func slug(h string) string {
	h = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(h), "`", ""))
	return strings.ReplaceAll(nonSlugChar.ReplaceAllString(h, ""), " ", "-")
}

// headings returns the slugs of all headings outside code fences and the H1 count.
func headings(t *testing.T, path string) (slugs []string, h1 int) {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec // repo doc path from a fixed list
	require.NoError(t, err)
	fenced := false
	for line := range strings.SplitSeq(string(raw), "\n") {
		if strings.HasPrefix(line, "```") {
			fenced = !fenced
			continue
		}
		if fenced || !strings.HasPrefix(line, "#") {
			continue
		}
		text := strings.TrimLeft(line, "#")
		if strings.HasPrefix(line, "# ") {
			h1++
		}
		slugs = append(slugs, slug(text))
	}
	return slugs, h1
}

func TestDocsIndexAndLinks(t *testing.T) {
	files, err := filepath.Glob("../../docs/*.md")
	require.NoError(t, err)
	var got []string
	for _, f := range files {
		got = append(got, strings.TrimSuffix(filepath.Base(f), ".md"))
	}
	slices.Sort(got)
	want := slices.Clone(docPages)
	slices.Sort(want)
	require.Equal(t, want, got, "docs page set changed")

	readme, err := os.ReadFile("../../README.md")
	require.NoError(t, err)
	anchors := map[string][]string{}
	for _, p := range docPages {
		assert.Contains(t, string(readme), "](docs/"+p+".md)", "README does not link %s", p)
		s, h1 := headings(t, "../../docs/"+p+".md")
		assert.Equal(t, 1, h1, "%s needs exactly one H1", p)
		anchors[p+".md"] = s
	}
	for page, list := range stableAnchors {
		for _, a := range list {
			assert.Contains(t, anchors[page], a, "%s lost anchor #%s", page, a)
		}
	}
	for _, p := range docPages {
		checkLinks(t, anchors, "../../docs/"+p+".md", "")
	}
	checkLinks(t, anchors, "../../README.md", "docs/")
}

// checkLinks verifies the relative .md links of src that start with prefix.
func checkLinks(t *testing.T, anchors map[string][]string, src, prefix string) {
	t.Helper()
	raw, err := os.ReadFile(src) //nolint:gosec // repo doc path from a fixed list
	require.NoError(t, err)
	for _, m := range mdLink.FindAllStringSubmatch(string(raw), -1) {
		target, frag, _ := strings.Cut(m[1], "#")
		page, ok := strings.CutPrefix(target, prefix)
		if !ok || !strings.HasSuffix(page, ".md") || strings.Contains(page, "/") || strings.Contains(page, ":") {
			continue // external, or outside the docs pages (for example maintainers)
		}
		list, found := anchors[page]
		if !assert.True(t, found, "%s links to missing page %s", src, m[1]) {
			continue
		}
		if frag != "" {
			assert.Contains(t, list, frag, "%s links to missing anchor %s", src, m[1])
		}
	}
}
