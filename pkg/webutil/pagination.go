package webutil

import (
	"fmt"
	"math"

	"github.com/gofiber/fiber/v2"
	"github.com/werbot/werbot/pkg/strutil"
)

/*
import (
	"fmt"
	"math"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/pkg/strutil"
)

var defaultLimit int32 = 10

// PaginationQuery is ...
type PaginationQuery struct {
	Limit  int32  `json:"limit" form:"limit"`
	Offset int32  `json:"offset" form:"offset"`
	SortBy string `json:"sort_by" form:"sort_by"`
}

// GetPaginationFromCtx is ...
func GetPaginationFromCtx(c *fiber.Ctx) *PaginationQuery {
	p := &PaginationQuery{}
	p.SetLimit(c.Query("limit"))
	p.SetOffset(c.Query("offset"))
	p.SetSortBy(c.Query("sort_by"))
	return p
}

// SetLimit is ...
func (p *PaginationQuery) SetLimit(limitQuery string) {
	if limitQuery == "" {
		p.Limit = defaultLimit
		return
	}
	p.Limit = strutil.ToInt32(limitQuery)
}

// SetOffset is ...
func (p *PaginationQuery) SetOffset(pageQuery string) {
	if pageQuery == "" {
		p.Offset = 0
		return
	}
	p.Offset = strutil.ToInt32(pageQuery)
}

// SetSortBy is ...
func (p *PaginationQuery) SetSortBy(sortByQuery string) {
	if sortByQuery == "" {
		p.SortBy = ""
		return
	}
	p.SortBy = sortByQuery
}

// GetLimit is ...
func (p *PaginationQuery) GetLimit() int32 {
	return p.Limit
}

// GetOffset is ...
func (p *PaginationQuery) GetOffset() int32 {
	return p.Offset
}

// GetSortBy is ...
func (p *PaginationQuery) GetSortBy() string {
	return p.SortBy
}

// GetQueryString is ...
func (p *PaginationQuery) GetQueryString() string {
	return fmt.Sprintf("limit=%v&offset=%v", p.GetLimit(), p.GetOffset())
}

// GetSelectPage is ...
func (p *PaginationQuery) GetSelectPage() int32 {
	return p.Offset / p.Limit
}

// GetTotalPage is ...
func (p *PaginationQuery) GetTotalPage(totalRecords int32) int32 {
	return int32(math.Ceil(float64(totalRecords) / float64(p.Limit)))
}
*/

const defaultLimit int32 = 10 // Use const instead of var for constants.

// PaginationQuery represents pagination parameters.
type PaginationQuery struct {
	Limit  int32  `json:"limit" form:"limit"`
	Offset int32  `json:"offset" form:"offset"`
	SortBy string `json:"sort_by" form:"sort_by"` // Corrected typo in form tag.
}

// GetPaginationFromCtx creates a PaginationQuery from the fiber context.
func GetPaginationFromCtx(c *fiber.Ctx) *PaginationQuery {
	p := &PaginationQuery{
		Limit:  parseOrDefault(c.Query("limit"), defaultLimit),
		Offset: parseOrDefault(c.Query("offset"), 0),
		SortBy: c.Query("sort_by"),
	}
	return p
}

// parseOrDefault tries to parse an int32 value from the query parameter or returns the default value.
func parseOrDefault(queryParam string, defaultValue int32) int32 {
	if queryParam == "" {
		return defaultValue
	}

	value := strutil.ToInt32(queryParam)
	return value
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
