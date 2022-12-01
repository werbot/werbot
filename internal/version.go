package internal

import (
	"fmt"
	"runtime"
)

var (
	gitCommit = "00000000"
	version   = "0.0.1"
	buildDate = "28.09.2018"
	goVersion = runtime.Version()
	osArch    = fmt.Sprintf("%s %s", runtime.GOOS, runtime.GOARCH)
)

// Version returns the main version number that is being run at the moment.
func Version() string {
	return version
}

// Commit returns the git commit that was compiled. This will be filled in by the compiler.
func Commit() string {
	return gitCommit
}

// BuildDate returns the date the binary was built
func BuildDate() string {
	return buildDate
}

// GoVersion returns the version of the go runtime used to compile the binary
func GoVersion() string {
	return goVersion
}

// OsArch returns the os and arch used to build the binary
func OsArch() string {
	return osArch
}
