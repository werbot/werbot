package crypto

/*
import (
	"fmt"

	"github.com/werbot/werbot/internal/crypto"
)

func main() {
	//bytes := make([]byte, 32) //generate a random 32 byte key for AES-256
	//if _, err := rand.Read(bytes); err != nil {
	//	panic(err.Error())
	//}

	//key := hex.EncodeToString(bytes) //encode key in bytes to string and keep as secret, put in a vault
	key := "debbd278f60e6c46ecee2c419c571ba10e5a3ab391e88f397a5ec746379fb8bc"
	fmt.Printf("key to encrypt/decrypt : %s\n", key)

	encrypted := crypto.TextEncrypt("hello", key)
	fmt.Printf("encrypted : %s\n", encrypted)

	decrypted := crypto.TextDecrypt(encrypted, key)
	fmt.Printf("decrypted : %s\n", decrypted)
}
*/

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

// Encrypt is ....
func encrypt(stringToEncrypt, keyString string) string {
	// Since the key is in string, we need to convert decode it to bytes
	key, _ := hex.DecodeString(keyString)
	plaintext := []byte(stringToEncrypt)

	// Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}

	// Create a new GCM - https://en.wikipedia.org/wiki/Galois/Counter_Mode
	// https://golang.org/pkg/crypto/cipher/#NewGCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	// Create a nonce. Nonce should be from GCM
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err.Error())
	}

	// Encrypt the data using aesGCM.Seal
	// Since we don't want to save the nonce somewhere else in this case, we add it as a prefix to the encrypted data. The first nonce argument in Seal is the prefix.
	cipherText := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return fmt.Sprintf("%x", cipherText)
}

// Decrypt is ...
func decrypt(encryptedString, keyString string) string {
	key, _ := hex.DecodeString(keyString)
	enc, _ := hex.DecodeString(encryptedString)

	// Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}

	// Create a new GCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	// Get the nonce size
	nonceSize := aesGCM.NonceSize()

	// Extract the nonce from the encrypted data
	nonce, cipherText := enc[:nonceSize], enc[nonceSize:]

	// Decrypt the data
	plaintext, err := aesGCM.Open(nil, nonce, cipherText, nil)
	if err != nil {
		panic(err.Error())
	}

	return string(plaintext)
}
