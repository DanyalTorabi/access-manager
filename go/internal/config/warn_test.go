package config

import (
	"strings"
	"testing"
)

func TestAPIAuthStartupWarning(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		cfg  Config
		want string // empty = no warning
	}{
		{
			name: "token set",
			cfg:  Config{HTTPAddr: "0.0.0.0:8080", APIBearerToken: "secret"},
			want: "",
		},
		{
			name: "token whitespace only treated as unset warns",
			cfg:  Config{HTTPAddr: "0.0.0.0:8080", APIBearerToken: "  \t  "},
			want: "not loopback",
		},
		{
			name: "loopback IP",
			cfg:  Config{HTTPAddr: "127.0.0.1:8080"},
			want: "",
		},
		{
			name: "IPv6 loopback",
			cfg:  Config{HTTPAddr: "[::1]:8080"},
			want: "",
		},
		{
			name: "localhost hostname",
			cfg:  Config{HTTPAddr: "localhost:9000"},
			want: "",
		},
		{
			name: "all interfaces IPv4",
			cfg:  Config{HTTPAddr: "0.0.0.0:8080"},
			want: "not loopback",
		},
		{
			name: "port only addr",
			cfg:  Config{HTTPAddr: ":8080"},
			want: "all interfaces",
		},
		{
			name: "private IP",
			cfg:  Config{HTTPAddr: "10.0.0.1:8080"},
			want: "not loopback",
		},
		{
			name: "unparseable addr without leading colon",
			cfg:  Config{HTTPAddr: "not-a-hostport"},
			want: "",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := APIAuthStartupWarning(tc.cfg)
			switch tc.want {
			case "":
				if got != "" {
					t.Fatalf("expected no warning, got %q", got)
				}
			default:
				if !strings.Contains(got, tc.want) {
					t.Fatalf("expected warning containing %q, got %q", tc.want, got)
				}
			}
		})
	}
}
