package postgres

import (
	"fmt"
	"strings"
)

// QueryParse is parse query to map
// examples query:
// `parent=folders/123 AND lifecycleState=ACTIVE`
// `parent=folders/123`
// `displayName=\\"Test String\\"`
// `displayName="Test String"`
// `displayName='Test String'`
func (db *Connect) QueryParse(query string) map[string]string {
	ss := strings.Split(query, " AND ")
	m := make(map[string]string)
	for _, pair := range ss {
		z := strings.Split(pair, "=")
		z[1] = strings.ReplaceAll(z[1], `\\"`, "")
		z[1] = strings.ReplaceAll(z[1], `"`, "")
		z[1] = strings.ReplaceAll(z[1], `'`, "")

		z[0] = strings.TrimSpace(z[0])
		z[1] = strings.TrimSpace(z[1])

		m[z[0]] = z[1]
	}
	return m
}

// SQLAddWhere is ...
// example query's:
// name LIKE '%user1%' OR name LIKE '%user1%' OR email LIKE '%user1%'
// enable=true
// name LIKE '%user1%'
func (db *Connect) SQLAddWhere(query string) string {
	if query != "" {
		return fmt.Sprintf(" WHERE %s", query)
	}
	return ""
}

// SQLPagination is ...
// example query's for sortBy - id:DESC or id:ASC
func (db *Connect) SQLPagination(limit, offset int32, sortBy string) string {
	showLimit := 30
	showOffset := 0
	showSortBy := ""

	if offset > 0 {
		showOffset = int(offset)
	}
	if limit > 0 {
		showLimit = int(limit)
	}

	if sortBy != "" {
		sortBy := strings.SplitN(sortBy, ":", 2)
		showSortBy = fmt.Sprintf("ORDER BY %s %s", sortBy[0], sortBy[1])
	}

	return fmt.Sprintf(" %s LIMIT %v OFFSET %v", showSortBy, showLimit, showOffset)
}
