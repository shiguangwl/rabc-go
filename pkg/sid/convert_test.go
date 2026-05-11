package sid

import "testing"

func TestUint64ToBase62(t *testing.T) {
	tests := []struct {
		name string
		in   uint64
		want string
	}{
		{name: "zero", in: 0, want: "0"},
		{name: "base boundary", in: 61, want: "Z"},
		{name: "carry", in: 62, want: "10"},
		{name: "uint64 max", in: ^uint64(0), want: "lYGhA16ahyf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Uint64ToBase62(tt.in); got != tt.want {
				t.Fatalf("Uint64ToBase62(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
