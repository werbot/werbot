package test

// migrationDirs returns the schema migration directories for the core schema
// plus every add-on. The core directories are relative to the calling test
// package; add-on directories are passed with a trailing slash.
func migrationDirs(root string, addonDirs []string) []string {
	migrationsDirs := []string{root + "migration"}

	for _, dir := range addonDirs {
		migrationsDirs = append(migrationsDirs, dir+"migration")
	}

	return migrationsDirs
}
