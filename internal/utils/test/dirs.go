package test

// migrationDirs builds the goose migration and fixture directories for the
// core schema plus every add-on. The core directories are relative to the
// calling test package; add-on directories are passed with a trailing slash.
func migrationDirs(root string, addonDirs []string) []string {
	migrationsDirs := []string{root + "migration"}
	fixturesDirs := []string{root + "fixtures/migration"}

	for _, dir := range addonDirs {
		migrationsDirs = append(migrationsDirs, dir+"migration")
		fixturesDirs = append(fixturesDirs, dir+"fixtures/migration")
	}

	return append(migrationsDirs, fixturesDirs...)
}

// migrationDirsOnly builds the goose migration directories for the core schema
// plus every add-on, without fixture directories. The core directories are
// relative to the calling test package; add-on directories are passed with a
// trailing slash.
func migrationDirsOnly(root string, addonDirs []string) []string {
	migrationsDirs := []string{root + "migration"}

	for _, dir := range addonDirs {
		migrationsDirs = append(migrationsDirs, dir+"migration")
	}

	return migrationsDirs
}
