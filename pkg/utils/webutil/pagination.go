package webutil

import (
	"fmt"
	"math"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/pkg/utils/strutil"
)

const defaultLimit int32 = 25 // Use const instead of var for constants.

// PaginationQuery represents pagination parameters.
type PaginationQuery struct {
	Limit  int32  `json:"limit" form:"limit"`
	Offset int32  `json:"offset" form:"offset"`
	SortBy string `json:"sort_by" form:"sort_by"` // Corrected typo in form tag.
}

// GetPaginationFromCtx creates a PaginationQuery from the fiber context.
func GetPaginationFromCtx(c *fiber.Ctx) *PaginationQuery {
	return &PaginationQuery{
		Limit:  parseOrDefault(c.Query("limit"), defaultLimit),
		Offset: parseOrDefault(c.Query("offset"), 0),
		SortBy: c.Query("sort_by"),
	}
}

// parseOrDefault tries to parse an int32 value from the query parameter or returns the default value.
func parseOrDefault(queryParam string, defaultValue int32) int32 {
	if queryParam == "" {
		return defaultValue
	}
	return strutil.ToInt32(queryParam)
}

// GetQueryString constructs a query string with the pagination parameters.
func (p *PaginationQuery) GetQueryString() string {
	return fmt.Sprintf("limit=%d&offset=%d", p.Limit, p.Offset)
}

// GetSelectPage calculates the current page based on offset and limit.
func (p *PaginationQuery) GetSelectPage() int32 {
	if p.Limit == 0 {
		return 0
	}
	return p.Offset / p.Limit
}

// GetTotalPages calculates the total number of pages based on total records.
func (p *PaginationQuery) GetTotalPages(totalRecords int32) int32 {
	if p.Limit == 0 {
		return 0
	}
	return int32(math.Ceil(float64(totalRecords) / float64(p.Limit)))
}
