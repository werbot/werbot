package vnet

import (
	"strings"

	"github.com/google/uuid"
)

const maxLabel = 63

// OrgLabel turns an organization name into a DNS label. A name with no usable
// characters falls back to the first 8 hex digits of the organization id.
func OrgLabel(name string, orgID uuid.UUID) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		} else {
			dash = true
		}
	}
	label := b.String()
	if len(label) > maxLabel {
		label = strings.TrimRight(label[:maxLabel], "-")
	}
	if label == "" {
		return strings.ReplaceAll(orgID.String(), "-", "")[:8]
	}
	return label
}
