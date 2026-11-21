package cli

import (
	"os/exec"
	"runtime"
)

// OpenURL opens url in the default browser of the OS.
func OpenURL(url string) error {
	c := browserCommand(runtime.GOOS, url)
	return exec.Command(c[0], c[1:]...).Start() //nolint:gosec // fixed opener binary, url is one argv entry, no shell
}

func browserCommand(goos, url string) []string {
	switch goos {
	case "darwin":
		return []string{"open", url}
	case "windows":
		return []string{"rundll32", "url.dll,FileProtocolHandler", url}
	}
	return []string{"xdg-open", url}
}
