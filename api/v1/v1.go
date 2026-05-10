package v1

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CtxBizErrKey 存放原始 err 链的 ctx key，供 ResponseLogMiddleware 在 5xx 时统一打日志。
// 避免 handler/middleware 各自重复 logger.Error，也避免遗漏。
const CtxBizErrKey = "biz_err"

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// HandleSuccess 成功响应快捷入口；data 为 nil 时回填空对象，避免前端拿到 null。
func HandleSuccess(ctx *gin.Context, data any) {
	WriteResponse(ctx, nil, data)
}

// WriteResponse 是错误/成功响应的唯一入口。
//
// 行为：
//   - err == nil：等同 HandleSuccess。
//   - err 链可 errors.As 解析出 *Error：使用其 HTTP 作为状态码、Code/Message 作为业务码与消息。
//   - 其他未识别错误：统一映射到 ErrInternalServerError（500）。
//
// 5xx 响应会把原始 err 暂存到 ctx，由 ResponseLogMiddleware 统一记录调用上下文，
// handler 因此不必各自打日志，避免重复刷屏与遗漏。
func WriteResponse(ctx *gin.Context, err error, data any) {
	if err == nil {
		if data == nil {
			data = map[string]any{}
		}
		ctx.JSON(http.StatusOK, Response{
			Code:    ErrSuccess.Code,
			Message: ErrSuccess.Message,
			Data:    data,
		})
		return
	}

	var e *Error
	if !errors.As(err, &e) {
		// 集中翻译常见底层 sentinel，避免一律落入 500：
		// gorm.ErrRecordNotFound 是 service 层最常见的"id 不存在"信号，
		// 应当让前端拿到 404 而非 500，便于做"刷新列表"等针对性提示。
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			e = ErrNotFound.WithCause(err)
		default:
			e = ErrInternalServerError.WithCause(err)
		}
	}

	if data == nil {
		data = map[string]any{}
	}

	if e.HTTP >= http.StatusInternalServerError {
		ctx.Set(CtxBizErrKey, err)
	}

	ctx.JSON(e.HTTP, Response{
		Code:    e.Code,
		Message: e.Message,
		Data:    data,
	})
}
