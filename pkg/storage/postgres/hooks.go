package postgres

import "strings"

// SQLColumnsNull is ...
func SQLColumnsNull(include, null bool, columns []string) string {
	if include || len(columns) == 0 {
		return ""
	}

	nullStatus := "NULL"
	if !null {
		nullStatus = "NOT NULL"
	}

	return `AND ` + strings.Join(columns, ` IS `+nullStatus+` AND `) + ` IS ` + nullStatus
}
