package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test_HashPassword is ...
func Test_HashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{
			name:     "test0_01",
			password: "user6@werbot.net",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hashPassword, _ := HashPassword(tt.password, 13)
			status := CheckPasswordHash(tt.password, hashPassword)
			assert.Truef(t, status, "Hash for password [%s] was incorrect", tt.password)
		})
	}
}
