package common

// DefaultLimit 是未指定 limit 时的每页条数，与前端表格默认 pageSize 一致
const DefaultLimit = 20

// PageParam 是分页查询参数，对应前端 PageParam；
// sort 为排序字段，order 为 asc/desc，多字段排序时前端会拼接成 "a asc,b desc"
type PageParam struct {
	Page  int    `form:"page" json:"page"`
	Limit int    `form:"limit" json:"limit"`
	Sort  string `form:"sort" json:"sort"`
	Order string `form:"order" json:"order"`
}

// Offset 返回查询偏移量，page 未设置时从第 1 页开始
func (p PageParam) Offset() int {
	if p.Page <= 1 {
		return 0
	}
	return (p.Page - 1) * p.PageSize()
}

// PageSize 返回每页条数，limit 未设置时使用 DefaultLimit
func (p PageParam) PageSize() int {
	if p.Limit <= 0 {
		return DefaultLimit
	}
	return p.Limit
}
