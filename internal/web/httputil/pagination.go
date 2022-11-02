package httputil

import (
	"fmt"
	"math"

	"github.com/gofiber/fiber/v2"
	"github.com/werbot/werbot/internal/utils/convert"
)

var defaultLimit int32 = 10

// PaginationQuery is ...
type PaginationQuery struct {
	Limit  int32  `json:"limit" form:"limit"`
	Offset int32  `json:"offset" form:"offset"`
	SortBy string `json:"sort_by" form:"sor_tby"`
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
	n := convert.StringToInt32(limitQuery)
	p.Limit = n
}

// SetOffset is ...
func (p *PaginationQuery) SetOffset(pageQuery string) {
	if pageQuery == "" {
		p.Offset = 0
		return
	}
	n := convert.StringToInt32(pageQuery)
	p.Offset = n
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
