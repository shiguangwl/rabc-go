package apiv1

// 分页默认值与上限。集中放在 api 层，避免 handler/service/repository 各自硬编码。
const (
	DefaultPage     = 1
	DefaultPageSize = 10
	MaxPageSize     = 100
)

// Pagination 通用分页参数。所有列表查询请求结构体应嵌入此类型。
//
// 校验分两层：
//   - handler 层：依赖 binding tag 拦下明显非法值（小于 1 / 超过 MaxPageSize）
//   - service 层：调用 Normalize 兜底缺省值与边界，repo 层据此直接构造 SQL
//
// 这样 repo 不再二次校验入参，符合"边界处一次性校验"的原则。
type Pagination struct {
	Page     int `form:"page" binding:"omitempty,min=1" example:"1"`
	PageSize int `form:"pageSize" binding:"omitempty,min=1,max=100" example:"10"`
}

// Normalize 将零值和越界值替换为安全默认值。在 service 入口调用一次即可。
func (p *Pagination) Normalize() {
	if p.Page < 1 {
		p.Page = DefaultPage
	}
	if p.PageSize <= 0 {
		p.PageSize = DefaultPageSize
	}
	if p.PageSize > MaxPageSize {
		p.PageSize = MaxPageSize
	}
}

// Offset 返回 SQL OFFSET。内部 Normalize 一次保证非负——
// 即使调用方漏调 Normalize（例：未来新增的 repo 方法绕过 service 层），也不会落到负 OFFSET。
func (p *Pagination) Offset() int {
	p.Normalize()
	return (p.Page - 1) * p.PageSize
}

// Limit 返回 SQL LIMIT。内部 Normalize 一次保证落在 [DefaultPageSize, MaxPageSize]。
func (p *Pagination) Limit() int {
	p.Normalize()
	return p.PageSize
}
