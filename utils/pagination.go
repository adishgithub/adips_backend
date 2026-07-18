package utils

import (
	"math"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Pagination struct {
	Page         int   `json:"page"`
	Limit        int   `json:"limit"`
	Offset       int   `json:"-"`
	TotalRecords int64 `json:"total_records"`
	TotalPages   int   `json:"total_pages"`
	HasNext      bool  `json:"has_next"`
	HasPrev      bool  `json:"has_prev"`
}

func GetPagination(c *gin.Context) Pagination {

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

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

	p.HasNext = p.Page < p.TotalPages
	p.HasPrev = p.Page > 1

	return p
}
