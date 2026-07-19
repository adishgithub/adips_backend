package utils

import "math"

type Pagination struct {
	Page         int   `json:"page"`
	Limit        int   `json:"limit"`
	Offset       int   `json:"-"`
	TotalRecords int64 `json:"total_records"`
	TotalPages   int   `json:"total_pages"`
	HasNext      bool  `json:"has_next"`
	HasPrev      bool  `json:"has_prev"`
}

// NewPagination builds a Pagination from already-parsed page/limit
// values, clamping to sane bounds. Parsing raw query params is a
// gin/HTTP concern and stays in the handler; this function is pure
// so it's usable (and testable) from the service layer too.
func NewPagination(page, limit int) Pagination {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	return Pagination{
		Page:   page,
		Limit:  limit,
		Offset: (page - 1) * limit,
	}
}

func BuildPagination(p Pagination, total int64) Pagination {
	p.TotalRecords = total
	p.TotalPages = int(math.Ceil(float64(total) / float64(p.Limit)))
	if p.TotalPages == 0 {
		p.TotalPages = 1
	}
	p.HasNext = p.Page < p.TotalPages
	p.HasPrev = p.Page > 1
	return p
}
