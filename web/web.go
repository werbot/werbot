// Package web embeds the built SvelteKit UI (web/build, produced by make web).
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:build
var build embed.FS

// FS returns the contents of web/build.
func FS() fs.FS {
	sub, _ := fs.Sub(build, "build") // fails only for an invalid path
	return sub
}
