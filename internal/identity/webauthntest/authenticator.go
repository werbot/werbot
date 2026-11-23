// Package webauthntest is a software WebAuthn authenticator for tests: an
// ES256 key with attestation "none" that answers the JSON options of
// go-webauthn with the JSON a browser would send. Import it only from
// _test.go files.
package webauthntest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/fxamacker/cbor/v2"
)

// Authenticator flags (WebAuthn §6.1).
const (
	flagUP = 0x01 // user present
	flagUV = 0x04 // user verified
	flagBE = 0x08 // backup eligible
	flagAT = 0x40 // attested credential data included
)

var b64 = base64.RawURLEncoding

// Authenticator holds one credential. Origin is put into the client data;
// RPID, when set, replaces the RP ID of the options; BackupEligible marks the
// credential as a synced passkey.
type Authenticator struct {
	Origin         string
	RPID           string
	BackupEligible bool

	t      testing.TB
	key    *ecdsa.PrivateKey
	id     []byte
	user   []byte
	next   uint32
	encode cbor.EncMode
}

// New returns an authenticator that signs for origin.
func New(t testing.TB, origin string) *Authenticator {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := cbor.CoreDetEncOptions().EncMode()
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 16)
	_, _ = rand.Read(id)
	return &Authenticator{Origin: origin, t: t, key: key, id: id, encode: enc}
}

// SetSignCount sets the signature counter the next assertion carries.
func (a *Authenticator) SetSignCount(n uint32) { a.next = n }

type options struct {
	PublicKey struct {
		Challenge string `json:"challenge"`
		RPID      string `json:"rpId"`
		RP        struct {
			ID string `json:"id"`
		} `json:"rp"`
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	} `json:"publicKey"`
}

// Create answers credential creation options (protocol.CredentialCreation
// JSON) with attestation "none" and sign count 0; the next assertion carries 1.
func (a *Authenticator) Create(opts []byte) []byte {
	o := a.parse(opts)
	user, err := b64.DecodeString(o.PublicKey.User.ID)
	if err != nil {
		a.t.Fatal(err)
	}
	a.user, a.next = user, 1
	cdj := a.clientData("webauthn.create", o.PublicKey.Challenge)
	authData := a.authData(o.PublicKey.RP.ID, flagUP|flagUV|flagAT, 0)
	authData = binary.BigEndian.AppendUint16(append(authData, make([]byte, 16)...), uint16(len(a.id))) //nolint:gosec // zero AAGUID, a 16-byte id
	authData = append(append(authData, a.id...), a.marshal(a.coseKey())...)
	att := a.marshal(map[string]any{"fmt": "none", "attStmt": map[string]any{}, "authData": authData})
	return a.response(map[string]string{"clientDataJSON": b64.EncodeToString(cdj), "attestationObject": b64.EncodeToString(att)})
}

// Get answers credential request options (protocol.CredentialAssertion JSON)
// with an assertion signed by the credential.
func (a *Authenticator) Get(opts []byte) []byte {
	o := a.parse(opts)
	cdj := a.clientData("webauthn.get", o.PublicKey.Challenge)
	authData := a.authData(o.PublicKey.RPID, flagUP|flagUV, a.next)
	a.next++
	digest := sha256.Sum256(cdj)
	signed := sha256.Sum256(append(authData, digest[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, a.key, signed[:])
	if err != nil {
		a.t.Fatal(err)
	}
	return a.response(map[string]string{"clientDataJSON": b64.EncodeToString(cdj), "authenticatorData": b64.EncodeToString(authData),
		"signature": b64.EncodeToString(sig), "userHandle": b64.EncodeToString(a.user)})
}

func (a *Authenticator) parse(opts []byte) options {
	var o options
	if err := json.Unmarshal(opts, &o); err != nil {
		a.t.Fatal(err)
	}
	return o
}

func (a *Authenticator) clientData(typ, challenge string) []byte {
	cdj, err := json.Marshal(map[string]any{"type": typ, "challenge": challenge, "origin": a.Origin, "crossOrigin": false})
	if err != nil {
		a.t.Fatal(err)
	}
	return cdj
}

// authData is the RP ID hash, the flags and the sign count (WebAuthn §6.1).
func (a *Authenticator) authData(rpID string, flags byte, count uint32) []byte {
	if a.RPID != "" {
		rpID = a.RPID
	}
	if a.BackupEligible {
		flags |= flagBE
	}
	h := sha256.Sum256([]byte(rpID))
	return binary.BigEndian.AppendUint32(append(h[:], flags), count)
}

// coseKey is the EC2 P-256 ES256 public key in COSE form (RFC 9053).
func (a *Authenticator) coseKey() map[int]any {
	pub, err := a.key.PublicKey.ECDH()
	if err != nil {
		a.t.Fatal(err)
	}
	raw := pub.Bytes() // 0x04 || x || y
	return map[int]any{1: 2, 3: -7, -1: 1, -2: raw[1:33], -3: raw[33:]}
}

func (a *Authenticator) marshal(v any) []byte {
	b, err := a.encode.Marshal(v)
	if err != nil {
		a.t.Fatal(err)
	}
	return b
}

func (a *Authenticator) response(r map[string]string) []byte {
	id := b64.EncodeToString(a.id)
	b, err := json.Marshal(map[string]any{"id": id, "rawId": id, "type": "public-key", "response": r})
	if err != nil {
		a.t.Fatal(err)
	}
	return b
}
