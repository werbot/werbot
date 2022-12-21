package license

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"time"

	"golang.org/x/crypto/ed25519"
)

var (
	// ErrLicenseKeyIsBroken is ...
	ErrLicenseKeyIsBroken = errors.New("The license key has a broken")

	// ErrLicenseStructureIsBroken is ...
	ErrLicenseStructureIsBroken = errors.New("The license has a broken structure")

	// ErrFailedToOpenLicenseFile is ...
	ErrFailedToOpenLicenseFile = errors.New("Failed to open license file")
)

// Private is ...
type Private struct {
	key     ed25519.PrivateKey
	License *License
}

// Public is ...
type Public struct {
	key     ed25519.PublicKey
	License *License
}

// License is a ...
type License struct {
	Iss string          `json:"iss,omitempty"` // Issued By
	Cus string          `json:"cus,omitempty"` // Customer ID
	Sub string          `json:"sub,omitempty"` // Subscriber ID
	Typ string          `json:"typ,omitempty"` // License Type
	Ips string          `json:"ips,omitempty"` // Ips
	Iat time.Time       `json:"iat,omitempty"` // Issued At
	Exp time.Time       `json:"exp,omitempty"` // Expires At
	Dat json.RawMessage `json:"dat,omitempty"` // Data
}

// New is generate new license
func New(privateKey []byte) (*Private, error) {
	decodedPrivateKey, err := decodeKey(privateKey)
	if err != nil {
		return nil, err
	}

	return &Private{
		key: ed25519.PrivateKey(decodedPrivateKey),
	}, nil
}

// Read and decode license
func Read(publicKey []byte) (*Public, error) {
	decodedPublicKey, err := decodeKey(publicKey)
	if err != nil {
		return nil, err
	}

	return &Public{
		key: ed25519.PublicKey(decodedPublicKey),
	}, nil
}

// Encode is a ...
func (l *Private) Encode() ([]byte, error) {
	if l.key == nil {
		return nil, errors.New("private key is not set")
	}

	msg, err := json.Marshal(l.License)
	if err != nil {
		return nil, err
	}

	sig := ed25519.Sign(l.key, msg)
	buf := new(bytes.Buffer)
	buf.Write(sig)
	buf.Write(msg)

	block := &pem.Block{
		Type:  "LICENSE KEY",
		Bytes: buf.Bytes(),
	}
	return pem.EncodeToMemory(block), nil
}

// Decode is a ...
func (l *Public) Decode(data []byte) (*Public, error) {
	if l.key == nil {
		return nil, errors.New("public key is not set")
	}

	block, _ := pem.Decode(data)
	if block == nil || len(block.Bytes) < ed25519.SignatureSize {
		return nil, errors.New("Malformed License")
	}

	sig := block.Bytes[:ed25519.SignatureSize]
	msg := block.Bytes[ed25519.SignatureSize:]

	verified := ed25519.Verify(l.key, msg, sig)
	if !verified {
		return nil, errors.New("Invalid signature")
	}
	out := new(License)
	err := json.Unmarshal(msg, out)
	l.License = out
	return l, err
}

// Expired is a ...
func (l *Public) Expired() bool {
	return !l.License.Exp.IsZero() && time.Now().After(l.License.Exp)
}

// Info is ...
func (l *Public) Info() *License {
	return l.License
}

func decodeKey(b []byte) ([]byte, error) {
	enc := base64.StdEncoding
	buf := make([]byte, enc.DecodedLen(len(b)))
	n, err := enc.Decode(buf, b)
	return buf[:n], err
}
