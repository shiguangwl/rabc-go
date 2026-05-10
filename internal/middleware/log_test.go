package middleware

import "testing"

func TestMaskURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "no query", raw: "/v1/users", want: "/v1/users"},
		{name: "mask sensitive query", raw: "/cb?code=abc&state=ok&accessToken=t", want: "/cb?accessToken=%2A%2A%2A&code=%2A%2A%2A&state=ok"},
		{name: "invalid query", raw: "/cb?bad=%zz", want: "/cb?[query omitted]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maskURL(tt.raw); got != tt.want {
				t.Fatalf("maskURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMaskBody(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "json object", body: `{"username":"admin","password":"secret"}`, want: `{"password":"***","username":"admin"}`},
		{name: "non json", body: "password=secret", want: "[non-json body omitted]"},
		{name: "invalid json", body: `{"password":`, want: "[non-json body omitted]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maskBody([]byte(tt.body), 1024); got != tt.want {
				t.Fatalf("maskBody() = %q, want %q", got, tt.want)
			}
		})
	}
}
