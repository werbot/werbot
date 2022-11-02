package crypto

// CheckAESKey is ...
func CheckAESKey(aesKey string) bool {
	switch len(aesKey) {
	case 0, 16, 24, 32:
	default:
		return false
	}
	return true
}

// TextEncrypt is ...
func TextEncrypt(cryptoText, aesKey string) string {
	if len(cryptoText) != 0 && len(aesKey) != 0 {
		cryptoText = encrypt(cryptoText, aesKey)
	}
	return cryptoText
}

// TextDecrypt is ...
func TextDecrypt(cryptoText, aesKey string) string {
	if len(cryptoText) != 0 && len(aesKey) != 0 {
		cryptoText = decrypt(cryptoText, aesKey)
	}
	return cryptoText
}
