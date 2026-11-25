package vnet

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
)

const maxLabel = 63

// nonLabel is a run of characters a DNS label of OrgLabel never holds.
var nonLabel = regexp.MustCompile("[^a-z0-9]+")

// OrgLabel turns an organization name into a DNS label. A name with no usable
// characters falls back to the first 8 hex digits of the organization id.
func OrgLabel(name string, orgID uuid.UUID) string {
	label := strings.Trim(nonLabel.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if len(label) > maxLabel {
		label = strings.TrimRight(label[:maxLabel], "-")
	}
	if label == "" {
		return strings.ReplaceAll(orgID.String(), "-", "")[:8]
	}
	return label
}
