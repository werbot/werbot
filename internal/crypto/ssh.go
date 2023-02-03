package crypto

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"math/big"

	"golang.org/x/crypto/ssh"
)

var (
	// MsgFailedCreatingSSHKey is ...
	MsgFailedCreatingSSHKey = "failed to creating SSH key"

	// ErrFailedCreatingSSHKey is ...
	ErrFailedCreatingSSHKey = errors.New(MsgFailedCreatingSSHKey)
)

// PairOfKeys is ...
type PairOfKeys struct {
	KeyType    string
	PublicKey  []byte
	PrivateKey []byte
	Passphrase string
}

// NewSSHKey is ...
func NewSSHKey(keyType string) (*PairOfKeys, error) {
	var err error
	var passphrase string
	var block *pem.Block
	var pubKey ssh.PublicKey

	key := PairOfKeys{
		KeyType: keyType,
	}

	switch keyType {
	case "rsa":
		block, pubKey, err = newRSAKey()
		if err != nil {
			return nil, err
		}

		passphrase = NewPassword(22, false)

		block, err = x509.EncryptPEMBlock(rand.Reader, block.Type, block.Bytes, []byte(passphrase), x509.PEMCipherAES256)
		if err != nil {
			return nil, err
		}
	case "ed25519":
		block, pubKey, err = newEd25519Key()
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("key type not supported: %q, supported types are: rsa and ed25519", key.KeyType)
	}

	buf := bytes.NewBuffer(nil)
	if err := pem.Encode(buf, block); err != nil {
		return nil, err
	}

	return &PairOfKeys{
		PublicKey:  ssh.MarshalAuthorizedKey(pubKey),
		PrivateKey: buf.Bytes(),
		Passphrase: passphrase,
	}, nil
}

func newRSAKey() (*pem.Block, ssh.PublicKey, error) {
	// Generate keys
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}

	// Prepare public key
	pubKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, err
	}

	// Encode PEM
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	return block, pubKey, err
}

func newEd25519Key() (*pem.Block, ssh.PublicKey, error) {
	// Generate keys
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	// Prepare public key
	pubKey, err := ssh.NewPublicKey(privateKey.Public())
	if err != nil {
		return nil, nil, err
	}

	// Encode PEM
	block := &pem.Block{
		Type:  "OPENSSH PRIVATE KEY",
		Bytes: MarshalED25519PrivateKey(privateKey),
	}

	return block, pubKey, err
}

// Copied from https://github.com/mikesmitty/edkey/blob/master/edkey.go
/* Writes ed25519 private keys into the new OpenSSH private key format.
I have no idea why this isn't implemented anywhere yet, you can do seemingly
everything except write it to disk in the OpenSSH private key format. */

// MarshalED25519PrivateKey writes ed25519 private keys into the new OpenSSH private key format.
func MarshalED25519PrivateKey(key ed25519.PrivateKey) []byte {
	// Add our key header (followed by a null byte)
	magic := append([]byte("openssh-key-v1"), 0)

	var w struct {
		CipherName   string
		KdfName      string
		KdfOpts      string
		NumKeys      uint32
		PubKey       []byte
		PrivKeyBlock []byte
	}

	// Fill out the private key fields
	pk1 := struct {
		Check1  uint32
		Check2  uint32
		KeyType string
		Pub     []byte
		Priv    []byte
		Comment string
		Pad     []byte `ssh:"rest"`
	}{}

	// Set our check ints
	// ci := mathRand.Uint32()
	mathRand, _ := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	ci := uint32(mathRand.Uint64())
	pk1.Check1 = ci
	pk1.Check2 = ci

	// Set our key type
	pk1.KeyType = ssh.KeyAlgoED25519

	// Add the pubKey to the optionally-encrypted block
	pk, ok := key.Public().(ed25519.PublicKey)
	if !ok {
		// fmt.Fprintln(os.Stderr, "ed25519.PublicKey type assertion failed on an ed25519 public key. This should never ever happen.")
		return nil
	}
	pubKey := []byte(pk)
	pk1.Pub = pubKey

	// Add our private key
	pk1.Priv = []byte(key)

	// Might be useful to put something in here at some point
	pk1.Comment = ""

	// Add some padding to match the encryption block size within PrivKeyBlock (without Pad field)
	// 8 doesn't match the documentation, but that's what ssh-keygen uses for unencrypted keys. *shrug*
	bs := 8
	blockLen := len(ssh.Marshal(pk1))
	padLen := (bs - (blockLen % bs)) % bs
	pk1.Pad = make([]byte, padLen)

	// Padding is a sequence of bytes like: 1, 2, 3...
	for i := 0; i < padLen; i++ {
		pk1.Pad[i] = byte(i + 1)
	}

	// Generate the pubkey prefix "\0\0\0\nssh-ed25519\0\0\0 "
	prefix := []byte{0x0, 0x0, 0x0, 0x0b}
	prefix = append(prefix, []byte(ssh.KeyAlgoED25519)...)
	prefix = append(prefix, []byte{0x0, 0x0, 0x0, 0x20}...)

	// Only going to support unencrypted keys for now
	w.CipherName = "none"
	w.KdfName = "none"
	w.KdfOpts = ""
	w.NumKeys = 1
	w.PubKey = append(prefix, pubKey...)
	w.PrivKeyBlock = ssh.Marshal(pk1)

	magic = append(magic, ssh.Marshal(w)...)

	return magic
}
