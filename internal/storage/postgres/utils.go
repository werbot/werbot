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
	if offset < 0 {
		offset = 0
	}

	if limit <= 0 {
		limit = 30
	}

	showSortBy := ""
	if len(sortBy) > 0 {
		showSortBy = "ORDER BY "

		var orderParts []string
		sorts := strings.Split(sortBy, ",")
		for _, sort := range sorts {
			parts := strings.SplitN(sort, ":", 2)
			if len(parts) == 2 {
				orderParts = append(orderParts, fmt.Sprintf("%s %s", parts[0], parts[1]))
			}
		}
		showSortBy = showSortBy + strings.Join(orderParts, ", ")
	}

	return fmt.Sprintf(" %s LIMIT %d OFFSET %d", showSortBy, limit, offset)
}
