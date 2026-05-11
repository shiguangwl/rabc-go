package v1

// Error 应用层错误类型，承载业务码与 HTTP 状态码。
//
// 设计要点：
//   - Code 是给前端的业务码（前端按 code 分支处理），HTTP 是协议层状态码（由 WriteResponse 自动映射）。
//     handler 因此不再硬编码 http.StatusXxx，避免 service 已返回 ErrBadRequest 却被 handler 覆盖成 500。
//   - Is 按 Code 相等判定：sentinel 经 WithCause 包装、或被上层 fmt.Errorf("...: %w", err) 进一步包装后，
//     errors.Is(wrapped, v1.ErrXxx) 仍可命中，彻底替代旧版按指针查 map 的脆弱方式。
//   - WithCause 返回带原因的副本（不修改原 sentinel），用于在保留业务码的同时附带底层错误链供日志使用。
type Error struct {
	Code    int
	HTTP    int
	Message string
	cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.cause != nil {
		return e.Message + ": " + e.cause.Error()
	}
	return e.Message
}

// Unwrap 暴露底层 cause 供 errors.Is/As 沿链穿透。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Is 按业务 Code 相等判定，使包装后的错误仍能被 errors.Is 命中。
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e != nil && t != nil && e.Code != 0 && e.Code == t.Code
}

// WithCause 返回带 cause 的副本，原 sentinel 保持不变。
func (e *Error) WithCause(cause error) *Error {
	if e == nil {
		return nil
	}
	cp := *e
	cp.cause = cause
	return &cp
}

func newError(code, httpStatus int, msg string) *Error {
	return &Error{Code: code, HTTP: httpStatus, Message: msg}
}

// 业务码保留兼容（前端已在用）：
//   - 0/400/401/403/404/409/500 这套继续作为 Code 暴露给前端
//   - HTTP 字段由 WriteResponse 用作 ctx.JSON 的状态码
var (
	ErrSuccess             = newError(0, 200, "ok")
	ErrBadRequest          = newError(400, 400, "参数错误")
	ErrUnauthorized        = newError(401, 401, "登录失效，请重新登录~")
	ErrForbidden           = newError(403, 403, "权限不足，请联系管理员开通权限~")
	ErrNotFound            = newError(404, 404, "数据不存在")
	ErrConflict            = newError(409, 409, "资源已存在")
	ErrInternalServerError = newError(500, 500, "服务器错误~")
	ErrNotImplemented      = newError(501, 501, "接口未实现")

	// 业务扩展错误：业务码独立编号（>=1001），HTTP 码按语义映射
	ErrUsernameAlreadyUse = newError(1001, 409, "用户名已被占用")
	ErrRoleSidExists      = newError(1002, 409, "角色 sid 已存在")
	ErrRoleNameExists     = newError(1003, 409, "角色名已存在")
)
