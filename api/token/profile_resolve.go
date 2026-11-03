package token

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	"github.com/werbot/werbot/internal/trace"
)

const maxProfileAliasLength = 32

// resolveProfileID returns the profile attached to the invite, or creates it
// from the token metadata. If a profile with the invite email already exists,
// that profile is reused instead of failing the accept flow.
func (h *Handler) resolveProfileID(ctx context.Context, tokenData *tokenmessage.Token_Response, password, name, surname string) (string, error) {
	if profileID := tokenData.GetProfileId(); profileID != "" {
		return profileID, nil
	}

	email, metaName, metaSurname := tokenMetadata(tokenData)
	if email == "" {
		return "", status.Error(codes.InvalidArgument, trace.MsgTokenIsInvalid)
	}
	if name == "" {
		name = metaName
	}
	if surname == "" {
		surname = metaSurname
	}
	// Scheme invites carry only an email. Use a valid placeholder profile name
	// so a new user can register with just a password.
	if len(name) < 3 {
		name = "User"
	}
	if len(surname) < 3 {
		surname = "User"
	}

	existing, err := h.profileByEmail(ctx, email)
	if err == nil && existing.GetProfileId() != "" {
		return existing.GetProfileId(), nil
	}
	if err != nil && status.Code(err) != codes.NotFound {
		return "", err
	}

	created, err := h.addProfile(ctx, email, name, surname, password)
	if err != nil {
		// A concurrent request may have created the profile between the lookup
		// and the insert.
		if existing, lookupErr := h.profileByEmail(ctx, email); lookupErr == nil && existing.GetProfileId() != "" {
			return existing.GetProfileId(), nil
		}
		return "", err
	}

	return created.GetProfileId(), nil
}

// tokenMetadata returns the email and profile names stored in a token.
// Scheme tokens only carry an email.
func tokenMetadata(tokenData *tokenmessage.Token_Response) (email, name, surname string) {
	switch tokenData.GetSection() {
	case tokenenum.Section_profile, tokenenum.Section_project:
		profile := tokenData.GetProfile()
		return profile.GetEmail(), profile.GetName(), profile.GetSurname()
	case tokenenum.Section_scheme:
		return tokenData.GetScheme().GetEmail(), "", ""
	default:
		return "", "", ""
	}
}

// profileAlias derives an alias from the email local part: lowercase ASCII
// alphanumeric characters, at most 32 characters.
func profileAlias(email string) string {
	localPart := email
	if index := strings.IndexByte(email, '@'); index >= 0 {
		localPart = email[:index]
	}

	localPart = strings.ToLower(localPart)

	var builder strings.Builder
	for _, char := range localPart {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			builder.WriteRune(char)
		}
	}

	alias := builder.String()
	if alias == "" {
		alias = "user"
	}
	if len(alias) > maxProfileAliasLength {
		alias = alias[:maxProfileAliasLength]
	}
	return alias
}
