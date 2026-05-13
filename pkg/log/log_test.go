package log

import (
	"strings"
	"testing"

	"go.uber.org/zap/zapcore"
)

type stringArrayEncoder struct {
	values []string
}

func (*stringArrayEncoder) AppendBool(bool)             {}
func (*stringArrayEncoder) AppendByteString([]byte)     {}
func (*stringArrayEncoder) AppendComplex128(complex128) {}
func (*stringArrayEncoder) AppendComplex64(complex64)   {}
func (*stringArrayEncoder) AppendFloat64(float64)       {}
func (*stringArrayEncoder) AppendFloat32(float32)       {}
func (*stringArrayEncoder) AppendInt(int)               {}
func (*stringArrayEncoder) AppendInt64(int64)           {}
func (*stringArrayEncoder) AppendInt32(int32)           {}
func (*stringArrayEncoder) AppendInt16(int16)           {}
func (*stringArrayEncoder) AppendInt8(int8)             {}
func (e *stringArrayEncoder) AppendString(value string) { e.values = append(e.values, value) }
func (*stringArrayEncoder) AppendUint(uint)             {}
func (*stringArrayEncoder) AppendUint64(uint64)         {}
func (*stringArrayEncoder) AppendUint32(uint32)         {}
func (*stringArrayEncoder) AppendUint16(uint16)         {}
func (*stringArrayEncoder) AppendUint8(uint8)           {}
func (*stringArrayEncoder) AppendUintptr(uintptr)       {}

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
