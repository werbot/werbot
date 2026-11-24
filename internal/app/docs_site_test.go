package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSiteSidebarCoversDocs(t *testing.T) {
	cfgRaw, err := os.ReadFile("../../site/astro.config.mjs")
	require.NoError(t, err)
	cfg := string(cfgRaw)

	pages, err := filepath.Glob("../../docs/*.md")
	require.NoError(t, err)
	require.NotEmpty(t, pages)
	for _, p := range pages {
		name := strings.TrimSuffix(filepath.Base(p), ".md")
		assert.Contains(t, cfg, "slug: '"+name+"'", "docs/%s.md missing from the site sidebar", name)
	}
	assert.NotContains(t, cfg, "maintainers", "internal pages must not reach the site")

	ignore, err := os.ReadFile("../../site/.gitignore")
	require.NoError(t, err)
	for _, want := range []string{"node_modules/", "dist/", ".astro/", "src/content/docs/*.md"} {
		assert.Contains(t, string(ignore), want)
	}
}
