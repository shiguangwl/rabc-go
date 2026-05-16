package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"syscall"

	"github.com/gin-gonic/gin"

	"rabc-go/api/apiv1"
)

const ctxPanicStackKey = "panic_stack"

// Recovery 将 panic 收敛到项目统一响应协议。
//
// panic 代表不可预期的程序缺陷，不能让原始细节透给前端；但必须保留 cause 与 stack
// 供日志链路排查。若响应头已经写出，HTTP 协议层已无法改写 body，只能中断后续处理。
func Recovery() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				err := panicError(rec)
				if isBrokenConnection(err) {
					_ = ctx.Error(err)
					ctx.Abort()
					return
				}

				ctx.Set(apiv1.CtxBizErrKey, err)
				ctx.Set(ctxPanicStackKey, debug.Stack())

				ctx.Abort()
				if ctx.Writer.Written() {
					return
				}
				apiv1.WriteResponse(ctx, apiv1.ErrInternalServerError.WithCause(err), nil)
			}
		}()

		ctx.Next()
	}
}

func panicError(rec any) error {
	if err, ok := rec.(error); ok {
		return err
	}
	return fmt.Errorf("panic: %v", rec)
}

func isBrokenConnection(err error) bool {
	return errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, http.ErrAbortHandler)
}
