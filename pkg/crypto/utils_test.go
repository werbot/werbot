package crypto

import (
	"testing"
)

// Test_Crypto is ...
func Test_Crypto(t *testing.T) {
	tests := []struct {
		name       string
		cryptoText string
		aesKey     string
		expected   bool
	}{
		{
			name:       "Encryption followed by decryption of the text using an AES key",
			cryptoText: "Hello word!",
			aesKey:     "debbd278f60e6c46ecee2c419c571ba10e5a3ab391e88f397a5ec746379fb8bc",
			expected:   true,
		},
		{
			name:       "Encryption followed by decryption of the text without using an AES key",
			cryptoText: "Hello word!",
			aesKey:     "",
			expected:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypt := TextEncrypt(tt.cryptoText, tt.aesKey)
			decrypt := TextDecrypt(encrypt, tt.aesKey)
			if (decrypt != tt.cryptoText) == tt.expected {
				t.Errorf("Test_Crypto() = %v, want %v", decrypt, tt.cryptoText)
			}
		})
	}
}
