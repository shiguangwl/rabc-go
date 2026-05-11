package log

import (
	"strings"
	"testing"

	"go.uber.org/zap/zapcore"
)

type stringArrayEncoder struct {
	values []string
}

func (e *stringArrayEncoder) AppendBool(bool)             {}
func (e *stringArrayEncoder) AppendByteString([]byte)     {}
func (e *stringArrayEncoder) AppendComplex128(complex128) {}
func (e *stringArrayEncoder) AppendComplex64(complex64)   {}
func (e *stringArrayEncoder) AppendFloat64(float64)       {}
func (e *stringArrayEncoder) AppendFloat32(float32)       {}
func (e *stringArrayEncoder) AppendInt(int)               {}
func (e *stringArrayEncoder) AppendInt64(int64)           {}
func (e *stringArrayEncoder) AppendInt32(int32)           {}
func (e *stringArrayEncoder) AppendInt16(int16)           {}
func (e *stringArrayEncoder) AppendInt8(int8)             {}
func (e *stringArrayEncoder) AppendString(value string)   { e.values = append(e.values, value) }
func (e *stringArrayEncoder) AppendUint(uint)             {}
func (e *stringArrayEncoder) AppendUint64(uint64)         {}
func (e *stringArrayEncoder) AppendUint32(uint32)         {}
func (e *stringArrayEncoder) AppendUint16(uint16)         {}
func (e *stringArrayEncoder) AppendUint8(uint8)           {}
func (e *stringArrayEncoder) AppendUintptr(uintptr)       {}

// 守护文件日志走 ShortCallerEncoder：一旦有人改成 FullCallerEncoder，
// 日志会暴露开发机绝对路径（含 $HOME/$GOPATH），既泄露环境又把列撑爆。
func TestJSONEncoderUsesShortCallerPath(t *testing.T) {
	encoder := &stringArrayEncoder{}
	caller := zapcore.NewEntryCaller(0, "/Users/timeho/go/pkg/mod/github.com/casbin/gorm-adapter/v3@v3.32.0/adapter.go", 492, true)

	jsonEncoderConfig().EncodeCaller(caller, encoder)

	if len(encoder.values) != 1 {
		t.Fatalf("期望写入 1 个 caller 字段，实际写入 %d 个", len(encoder.values))
	}
	if got := encoder.values[0]; strings.Contains(got, "/Users/timeho/") {
		t.Fatalf("caller 不应包含本机绝对路径: %q", got)
	}
	if got, want := encoder.values[0], "v3@v3.32.0/adapter.go:492"; got != want {
		t.Fatalf("caller = %q, want %q", got, want)
	}
}
