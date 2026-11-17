package session

import (
	"encoding/hex"
	"encoding/json"

	"github.com/google/uuid"
)

// Manifest is the signed summary of a finished recording. Fields are in
// alphabetical key order, so json.Marshal yields canonical JSON.
type Manifest struct {
	Bytes     int64  `json:"bytes"`
	Chunks    int    `json:"chunks"`
	FinalHash string `json:"final_hash"` // lowercase hex
	SessionID string `json:"session_id"`
}

// NewManifest builds the manifest of a recording.
func NewManifest(sessionID uuid.UUID, chunks int, bytes int64, finalHash []byte) Manifest {
	return Manifest{Bytes: bytes, Chunks: chunks, FinalHash: hex.EncodeToString(finalHash), SessionID: sessionID.String()}
}

// Canonical is the byte string that gets signed.
func (m Manifest) Canonical() []byte {
	b, _ := json.Marshal(m) // plain struct of scalars: cannot fail
	return b
}
