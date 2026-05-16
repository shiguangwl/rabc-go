package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"rabc-go/api/apiv1"
)

func TestRecoveryWritesUnifiedResponseOnPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Recovery())
	router.GET("/panic", func(*gin.Context) {
		panic("boom")
	})

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusInternalServerError)
	}

	var body apiv1.Response
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if body.Code != apiv1.ErrInternalServerError.Code {
		t.Fatalf("code = %d, want %d", body.Code, apiv1.ErrInternalServerError.Code)
	}
	if body.Message != apiv1.ErrInternalServerError.Message {
		t.Fatalf("message = %q, want %q", body.Message, apiv1.ErrInternalServerError.Message)
	}
}

func TestPanicErrorKeepsErrorValue(t *testing.T) {
	src := errors.New("db exploded")

	if got := panicError(src); !errors.Is(got, src) {
		t.Fatalf("panicError() = %v, want original error", got)
	}
}

func TestRecoverySkipsResponseOnBrokenConnection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Recovery())
	router.GET("/abort", func(*gin.Context) {
		panic(http.ErrAbortHandler)
	})

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/abort", nil)

	router.ServeHTTP(resp, req)

	if resp.Body.Len() != 0 {
		t.Fatalf("broken connection 不应写响应体: %q", resp.Body.String())
	}
}
