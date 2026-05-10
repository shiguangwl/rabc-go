package middleware

import (
	"bytes"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/cryptor"
	"github.com/duke-git/lancet/v2/random"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	v1 "nunu-layout-admin/api/v1"
	"nunu-layout-admin/pkg/log"
)

// defaultMaxLogBodyBytes 是写入日志的 body 字节上限默认值。
// 配置项 log.body.max_bytes 可覆盖；调用方在构造中间件时传入实际值。
// 截断后用 [truncated N bytes, masking skipped] 替换，避免大 body 撑爆日志、
// 拖慢 marshal，也避免截断中段把敏感字段以未脱敏形式写入日志。
const defaultMaxLogBodyBytes = 8 * 1024

// 敏感请求头：使用 http.CanonicalHeaderKey 后的精确匹配。
var sensitiveHeaders = map[string]struct{}{
	"Authorization": {},
	"Cookie":        {},
	"Set-Cookie":    {},
	"Sign":          {},
}

// 敏感 body 字段：匹配采用大小写不敏感（strings.EqualFold），
// 同时覆盖 camelCase 与 snake_case 两套常见命名风格。
var sensitiveBodyFields = []string{
	"password",
	"new_password", "newPassword",
	"old_password", "oldPassword",
	"access_token", "accessToken",
	"refresh_token", "refreshToken",
	"token",
	"secret",
	"api_key", "apiKey",
}

var sensitiveQueryFields = map[string]struct{}{
	"access_token":  {},
	"accessToken":   {},
	"api_key":       {},
	"apiKey":        {},
	"code":          {},
	"key":           {},
	"password":      {},
	"refresh_token": {},
	"refreshToken":  {},
	"secret":        {},
	"sign":          {},
	"signature":     {},
	"token":         {},
}

// resolveMaxBodyBytes 选取生效的 body 字节上限：配置 > 默认。
// 配置非正数视为"使用默认"，避免 0/负数把日志体彻底截没。
func resolveMaxBodyBytes(maxBytes int) int {
	if maxBytes <= 0 {
		return defaultMaxLogBodyBytes
	}
	return maxBytes
}

// RequestLogMiddleware 打印请求摘要。logBody=false 时跳过 body 读取与脱敏，
// 既避免 prod 把请求体写日志（数据合规），又省去无谓的 GetRawData/JSON 重序列化。
//
// maxBytes：单条 body 写入日志的字节上限，<=0 时取 defaultMaxLogBodyBytes。
// 这里也作为 io.LimitReader 的硬上限，防止恶意/超大 body 把整个 raw 数据吃进内存。
func RequestLogMiddleware(logger *log.Logger, logBody bool, maxBytes int) gin.HandlerFunc {
	limit := resolveMaxBodyBytes(maxBytes)
	return func(ctx *gin.Context) {
		// trace 标识：UUID 失败时回退到 crypto/rand 16 字节 hex；避免直接走时间戳兜底，
		// 否则同一时刻的并发请求会拿到不可区分的 trace，给排查制造盲区。
		uuid, err := random.UUIdV4()
		if err != nil {
			var buf [16]byte
			if _, e := cryptorand.Read(buf[:]); e == nil {
				uuid = hex.EncodeToString(buf[:])
			} else {
				// crypto/rand 也失败属系统级故障；用纳秒时间戳兜底总比让请求挂掉好。
				uuid = fmt.Sprintf("rand-fallback-%d", time.Now().UnixNano())
			}
		}
		trace := cryptor.Md5String(uuid)
		logger.WithValue(ctx, zap.String("trace", trace))
		logger.WithValue(ctx, zap.String("request_method", ctx.Request.Method))
		logger.WithValue(ctx, zap.Any("request_headers", maskHeaders(ctx.Request.Header)))
		logger.WithValue(ctx, zap.String("request_url", maskURL(ctx.Request.URL.String())))
		if logBody && ctx.Request.Body != nil && shouldLogBody(ctx.Request.Header.Get("Content-Type")) {
			// 旧实现走 ctx.GetRawData() 一次性读全量到内存，攻击者发 100MB body
			// 仍会落到 RAM。这里用 LimitReader 截到 limit+1 字节：超出阈值时既能
			// 识别"被截断"又不会把整段读完。
			limited := io.LimitReader(ctx.Request.Body, int64(limit)+1)
			bodyBytes, _ := io.ReadAll(limited)
			// 把读到的 + 未读完的 body 拼回去，业务 handler 仍能正常 BindJSON。
			// 注意：必须把 bodyBytes 完整（含可能多读出的 1 字节）回放，否则 handler 的
			// 解析会缺失一个字节。日志侧再单独按 limit 切片即可。
			ctx.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(bodyBytes), ctx.Request.Body))
			if len(bodyBytes) > limit {
				logger.WithValue(ctx, zap.String("request_params",
					fmt.Sprintf("[truncated at %d bytes, masking skipped]", limit)))
			} else {
				logger.WithValue(ctx, zap.String("request_params", maskBody(bodyBytes, limit)))
			}
		}
		logger.WithContext(ctx).Info("Request")
		ctx.Next()
	}
}

// ResponseLogMiddleware 打印响应摘要。logBody=false 时不再为响应分配镜像 buffer，
// 完全旁路 bodyLogWriter，省去内存与 marshal 开销。
func ResponseLogMiddleware(logger *log.Logger, logBody bool, maxBytes int) gin.HandlerFunc {
	limit := resolveMaxBodyBytes(maxBytes)
	return func(ctx *gin.Context) {
		startTime := time.Now()
		var blw *bodyLogWriter
		if logBody {
			blw = &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: ctx.Writer, limit: limit}
			ctx.Writer = blw
		}
		ctx.Next()
		duration := time.Since(startTime).String()
		fields := []zap.Field{zap.String("time", duration)}
		if logBody && blw != nil {
			var body string
			if blw.truncated {
				// 截断点是字节边界，buffer 内大概率是半截 JSON；走 maskBody
				// 解析失败会 fallback 到原文路径泄露敏感字段。直接占位符替代。
				body = fmt.Sprintf("[response truncated at write boundary, %d bytes captured, masking skipped]", blw.body.Len())
			} else {
				body = maskBody(blw.body.Bytes(), limit)
			}
			fields = append(fields, zap.String("response_body", body))
		}
		logger.WithContext(ctx).Info("Response", fields...)

		// 5xx 错误链统一在此记录，handler/service 不再各自打错误日志，避免重复刷屏。
		// v1.WriteResponse 在 5xx 路径会把原始 err（含 wrap 链）放入 ctx，这里读出后输出。
		// 兜底：若 status>=500 但 ctx 中无 biz_err（典型来自 gin.Recovery 处理的 panic），
		// 仍发一条带 trace 的 ERROR，让告警与排查不至于丢线索。
		if v, ok := ctx.Get(v1.CtxBizErrKey); ok {
			if e, ok := v.(error); ok {
				logger.WithContext(ctx).Error("request failed", zap.Error(e))
			}
		} else if ctx.Writer.Status() >= 500 {
			logger.WithContext(ctx).Error("request failed without biz_err (likely panic recovered by gin.Recovery; check stderr for stack)",
				zap.Int("status", ctx.Writer.Status()))
		}
	}
}

type bodyLogWriter struct {
	gin.ResponseWriter
	body      *bytes.Buffer
	limit     int
	truncated bool
}

// mirror 把字节镜像到日志 buffer，截断超过 limit 的部分。
// 与 Write/WriteString 共用，保证两条路径行为一致。
func (w *bodyLogWriter) mirror(b []byte) {
	if w.truncated {
		return
	}
	remaining := w.limit - w.body.Len()
	switch {
	case remaining >= len(b):
		w.body.Write(b)
	case remaining > 0:
		w.body.Write(b[:remaining])
		w.truncated = true
	default:
		w.truncated = true
	}
}

// Write 把响应写到下游的同时镜像一份到 body buffer，供日志输出。
// 镜像 buffer 受 limit 上限保护：超过后停止往 buffer 写入并设 truncated 标记，
// 避免大响应在日志路径占用 N 倍内存（下游响应仍按完整字节写出，不影响客户端）。
func (w *bodyLogWriter) Write(b []byte) (int, error) {
	w.mirror(b)
	return w.ResponseWriter.Write(b)
}

// WriteString gin 的 c.String / c.HTML 等会偏好 io.StringWriter 接口；
// 不显式实现会跳过镜像，导致这两类响应日志缺失。
func (w *bodyLogWriter) WriteString(s string) (int, error) {
	w.mirror([]byte(s))
	return w.ResponseWriter.WriteString(s)
}

// maskHeaders 返回请求头副本，敏感字段值替换为 "***"。
func maskHeaders(h http.Header) http.Header {
	masked := make(http.Header, len(h))
	for k, v := range h {
		if _, ok := sensitiveHeaders[http.CanonicalHeaderKey(k)]; ok {
			masked[k] = []string{"***"}
			continue
		}
		masked[k] = v
	}
	return masked
}

// maskBody 尝试将 body 当作 JSON 解析，命中敏感字段时替换为 "***"。
// 非 JSON 或解析失败时只记录占位符，避免调试开关误把表单密码或畸形 JSON 原文写入日志。
// 合法 JSON 会经历 unmarshal/marshal 一轮，大整数精度与字段顺序可能与原 body 不一致，仅用于日志。
//
// 超过 limit 的输入只输出 [truncated N bytes, masking skipped]，
// 不输出原始字节内容——避免把"碰巧落在尾部之外"的敏感字段以未脱敏的
// 头部片段形式写入日志。
func maskBody(body []byte, limit int) string {
	if len(body) > limit {
		return fmt.Sprintf("[truncated %d bytes, masking skipped]", len(body))
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return string(body)
	}
	switch trimmed[0] {
	case '{':
		var obj map[string]any
		if err := json.Unmarshal(trimmed, &obj); err != nil {
			return "[non-json body omitted]"
		}
		maskMap(obj)
		out, err := json.Marshal(obj)
		if err != nil {
			return "[non-json body omitted]"
		}
		return string(out)
	case '[':
		var arr []any
		if err := json.Unmarshal(trimmed, &arr); err != nil {
			return "[non-json body omitted]"
		}
		maskArray(arr)
		out, err := json.Marshal(arr)
		if err != nil {
			return "[non-json body omitted]"
		}
		return string(out)
	default:
		return "[non-json body omitted]"
	}
}

func maskURL(raw string) string {
	if raw == "" {
		return raw
	}
	parts := strings.SplitN(raw, "?", 2)
	if len(parts) != 2 {
		return raw
	}
	values, err := url.ParseQuery(parts[1])
	if err != nil {
		return parts[0] + "?[query omitted]"
	}
	for k := range values {
		if _, ok := sensitiveQueryFields[k]; ok || isSensitiveField(k) {
			values[k] = []string{"***"}
		}
	}
	return parts[0] + "?" + values.Encode()
}

func isSensitiveField(name string) bool {
	for _, f := range sensitiveBodyFields {
		if strings.EqualFold(name, f) {
			return true
		}
	}
	return false
}

func maskMap(m map[string]any) {
	for k, v := range m {
		if isSensitiveField(k) {
			m[k] = "***"
			continue
		}
		switch vv := v.(type) {
		case map[string]any:
			maskMap(vv)
		case []any:
			maskArray(vv)
		}
	}
}

func maskArray(arr []any) {
	for _, item := range arr {
		switch vv := item.(type) {
		case map[string]any:
			maskMap(vv)
		case []any:
			maskArray(vv)
		}
	}
}

// shouldLogBody 判断 body 是否适合写入日志：
// multipart 和二进制流体积大、JSON 无意义且解析会耗内存，直接跳过。
func shouldLogBody(contentType string) bool {
	if contentType == "" {
		return true
	}
	ct := strings.TrimSpace(contentType)
	if strings.HasPrefix(strings.ToLower(ct), "multipart/") {
		return false
	}
	if strings.HasPrefix(strings.ToLower(ct), "application/octet-stream") {
		return false
	}
	return true
}
