package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_migrationDirs(t *testing.T) {
	testCases := []struct {
		name      string
		root      string
		addonDirs []string
		want      []string
	}{
		{
			name: "core only",
			root: "../../",
			want: []string{"../../migration", "../../fixtures/migration"},
		},
		{
			name:      "core and addons",
			root:      "../../../",
			addonDirs: []string{"../../../add-on/saas/", "../../../add-on/other/"},
			want: []string{
				"../../../migration",
				"../../../add-on/saas/migration",
				"../../../add-on/other/migration",
				"../../../fixtures/migration",
				"../../../add-on/saas/fixtures/migration",
				"../../../add-on/other/fixtures/migration",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, migrationDirs(tt.root, tt.addonDirs))
		})
	}
}
