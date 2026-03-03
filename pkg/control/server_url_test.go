package control

import (
	"net"
	"testing"
)

func TestControlServerListenAddrAndDashboardURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		addr          net.Addr
		tlsEnabled    bool
		wantListen    string
		wantDashboard string
	}{
		{
			name:          "ipv4 loopback http",
			addr:          &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8376},
			wantListen:    "127.0.0.1:8376",
			wantDashboard: "http://127.0.0.1:8376/dashboard/",
		},
		{
			name:          "ipv4 wildcard rewrites to loopback",
			addr:          &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 3030},
			wantListen:    "0.0.0.0:3030",
			wantDashboard: "http://127.0.0.1:3030/dashboard/",
		},
		{
			name:          "ipv6 wildcard rewrites to loopback",
			addr:          &net.TCPAddr{IP: net.ParseIP("::"), Port: 4040},
			wantListen:    "[::]:4040",
			wantDashboard: "http://[::1]:4040/dashboard/",
		},
		{
			name:          "tls uses https",
			addr:          &net.TCPAddr{IP: net.ParseIP("192.168.1.50"), Port: 9443},
			tlsEnabled:    true,
			wantListen:    "192.168.1.50:9443",
			wantDashboard: "https://192.168.1.50:9443/dashboard/",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotListen, gotDashboard := controlServerListenAddrAndDashboardURL(tt.addr, tt.tlsEnabled)
			if gotListen != tt.wantListen {
				t.Fatalf("listen addr = %q, want %q", gotListen, tt.wantListen)
			}
			if gotDashboard != tt.wantDashboard {
				t.Fatalf("dashboard url = %q, want %q", gotDashboard, tt.wantDashboard)
			}
		})
	}
}
