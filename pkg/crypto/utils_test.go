package crypto

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func Test_TextEncryptDecrypt(t *testing.T) {
	const key = "debbd278f60e6c46ecee2c419c571ba10e5a3ab391e88f397a5ec746379fb8bc" // 32 bytes
	const otherKey = "00112233445566778899aabbccddeeff"                            // 16 bytes

	encrypted, err := TextEncrypt("Hello word!", key)
	if err != nil {
		t.Fatalf("TextEncrypt() error = %v", err)
	}
	decrypted, err := TextDecrypt(encrypted, key)
	if err != nil {
		t.Fatalf("TextDecrypt() error = %v", err)
	}
	if decrypted != "Hello word!" {
		t.Fatalf("TextDecrypt() = %q, want %q", decrypted, "Hello word!")
	}

	t.Run("empty plaintext is not encrypted", func(t *testing.T) {
		got, err := TextEncrypt("", key)
		if err != nil || got != "" {
			t.Fatalf("TextEncrypt(\"\") = (%q, %v), want (\"\", nil)", got, err)
		}
	})

	t.Run("empty ciphertext is rejected", func(t *testing.T) {
		_, err := TextDecrypt("", key)
		if !errors.Is(err, ErrInvalidCiphertext) {
			t.Fatalf("TextDecrypt(\"\") error = %v, want ErrInvalidCiphertext", err)
		}
	})

	t.Run("wrong key", func(t *testing.T) {
		_, err := TextDecrypt(encrypted, otherKey)
		if !errors.Is(err, ErrDecrypt) {
			t.Fatalf("TextDecrypt() error = %v, want ErrDecrypt", err)
		}
	})

	t.Run("bad hex ciphertext", func(t *testing.T) {
		_, err := TextDecrypt("zz-not-hex", key)
		if !errors.Is(err, ErrInvalidCiphertext) {
			t.Fatalf("TextDecrypt() error = %v, want ErrInvalidCiphertext", err)
		}
	})

	t.Run("short ciphertext", func(t *testing.T) {
		_, err := TextDecrypt("00", key)
		if !errors.Is(err, ErrInvalidCiphertext) {
			t.Fatalf("TextDecrypt() error = %v, want ErrInvalidCiphertext", err)
		}
	})

	t.Run("bad hex key", func(t *testing.T) {
		_, err := TextEncrypt("data", "not-hex-key")
		if !errors.Is(err, ErrInvalidKey) {
			t.Fatalf("TextEncrypt() error = %v, want ErrInvalidKey", err)
		}
		_, err = TextDecrypt(encrypted, "not-hex-key")
		if !errors.Is(err, ErrInvalidKey) {
			t.Fatalf("TextDecrypt() error = %v, want ErrInvalidKey", err)
		}
	})

	t.Run("empty key", func(t *testing.T) {
		_, err := TextEncrypt("data", "")
		if !errors.Is(err, ErrInvalidKey) {
			t.Fatalf("TextEncrypt() error = %v, want ErrInvalidKey", err)
		}
	})

	t.Run("wrong length key", func(t *testing.T) {
		// 16 hex characters decode to 8 bytes, which is not a valid AES key.
		_, err := TextEncrypt("data", strings.Repeat("ab", 8))
		if !errors.Is(err, ErrInvalidKey) {
			t.Fatalf("TextEncrypt() error = %v, want ErrInvalidKey", err)
		}
	})
}

func Test_CheckAESKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{name: "16 bytes", key: strings.Repeat("ab", 16), want: true},
		{name: "24 bytes", key: strings.Repeat("ab", 24), want: true},
		{name: "32 bytes", key: strings.Repeat("ab", 32), want: true},
		{name: "empty", key: "", want: false},
		{name: "16 characters is not 16 bytes", key: strings.Repeat("a", 16), want: false},
		{name: "not hex", key: strings.Repeat("zz", 16), want: false},
		{name: "wrong byte length", key: strings.Repeat("ab", 20), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckAESKey(tt.key); got != tt.want {
				t.Fatalf("CheckAESKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func Test_AESKeyGen(t *testing.T) {
	for _, length := range []int{16, 24, 32} {
		key, err := AESKeyGen(length)
		if err != nil {
			t.Fatalf("AESKeyGen(%d) error = %v", length, err)
		}
		if !CheckAESKey(key) {
			t.Fatalf("AESKeyGen(%d) = %q, not a valid AES key", length, key)
		}
		if decoded, err := hex.DecodeString(key); err != nil || len(decoded) != length {
			t.Fatalf("AESKeyGen(%d) decoded to %d bytes, err %v", length, len(decoded), err)
		}
	}

	for _, length := range []int{0, 1, 8, 15, 17, 64} {
		if _, err := AESKeyGen(length); !errors.Is(err, ErrInvalidKey) {
			t.Fatalf("AESKeyGen(%d) error = %v, want ErrInvalidKey", length, err)
		}
	}
}
