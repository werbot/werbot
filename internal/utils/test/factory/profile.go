package factory

import (
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/werbot/werbot/pkg/crypto"
)

// ProfileParams is the set of optional and required values for a profile.
type ProfileParams struct {
	ID, Alias, Name, Surname, Email, Password  string
	Role                                       int32
	Active, Confirmed                          *bool
	LockedAt, ArchivedAt, UpdatedAt, CreatedAt *time.Time
}

// Profile is a created profile row.
type Profile struct {
	ID, Alias, Name, Surname, Email, Password string
	Role                                      int32
	Active, Confirmed                         bool
}

// PublicKeyParams is the set of optional and required values for a profile
// public key.
type PublicKeyParams struct {
	ID, ProfileID, Title, Key, Fingerprint     string
	LockedAt, ArchivedAt, UpdatedAt, CreatedAt *time.Time
}

// PublicKey is a created profile public key row.
type PublicKey struct{ ID, ProfileID, Title, Key, Fingerprint string }

// PublicKey inserts one profile public key row and returns it.
func (b *Builder) PublicKey(p PublicKeyParams) PublicKey {
	b.t.Helper()

	p.ID = uuidOrNew(p.ID)
	if p.Title == "" {
		p.Title = "public_key"
	}
	if p.Key == "" {
		p.Key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI" + randomAlias()
	}
	if p.Fingerprint == "" {
		p.Fingerprint = randomAlias()
	}

	_, err := b.db.Exec(`
		INSERT INTO "profile_public_key" (
			"id", "profile_id", "title", "key", "fingerprint",
			"locked_at", "archived_at", "updated_at", "created_at"
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE($9, NOW()))
	`, p.ID, p.ProfileID, p.Title, p.Key, p.Fingerprint,
		p.LockedAt, p.ArchivedAt, p.UpdatedAt, p.CreatedAt)
	require.NoError(b.t, err, "public key: insert")

	return PublicKey{
		ID:          p.ID,
		ProfileID:   p.ProfileID,
		Title:       p.Title,
		Key:         p.Key,
		Fingerprint: p.Fingerprint,
	}
}

// Profile inserts one profile row and returns it.
func (b *Builder) Profile(p ProfileParams) Profile {
	b.t.Helper()

	p.ID = uuidOrNew(p.ID)
	if p.Alias == "" {
		p.Alias = randomAlias()
	}
	if p.Password == "" {
		p.Password = defaultPassword
	}
	if p.Role == 0 {
		p.Role = 1
	}
	active := boolOr(p.Active, true)
	confirmed := boolOr(p.Confirmed, true)

	hash, err := crypto.HashPassword(p.Password, bcrypt.MinCost)
	require.NoError(b.t, err, "profile: hash password")

	_, err = b.db.Exec(`
		INSERT INTO "profile" (
			"id", "alias", "name", "surname", "email", "password",
			"role", "active", "confirmed",
			"locked_at", "archived_at", "updated_at", "created_at"
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, COALESCE($13, NOW()))
	`, p.ID, p.Alias, p.Name, p.Surname, p.Email, hash, p.Role, active, confirmed,
		p.LockedAt, p.ArchivedAt, p.UpdatedAt, p.CreatedAt)
	require.NoError(b.t, err, "profile: insert")

	return Profile{
		ID:        p.ID,
		Alias:     p.Alias,
		Name:      p.Name,
		Surname:   p.Surname,
		Email:     p.Email,
		Password:  hash,
		Role:      p.Role,
		Active:    active,
		Confirmed: confirmed,
	}
}
