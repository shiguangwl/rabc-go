package repository

// PageQuery 是 repository 层通用分页参数，避免持久层依赖 HTTP DTO。
type PageQuery struct {
	Page     int
	PageSize int
}

func (q PageQuery) Offset() int {
	return (q.Page - 1) * q.PageSize
}

func (q PageQuery) Limit() int {
	return q.PageSize
}

type AdminUserQuery struct {
	PageQuery
	Username string
	Nickname string
	Email    string
	Phone    string
}

type RoleQuery struct {
	PageQuery
	Sid  string
	Name string
}

type APIQuery struct {
	PageQuery
	Group  string
	Name   string
	Path   string
	Method string
}
