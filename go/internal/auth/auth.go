package auth

import (
	"net"
	"net/http"
	"strings"
)

var privateSubnets = []net.IPNet{
	{IP: net.IPv4(10, 0, 0, 0), Mask: net.CIDRMask(8, 32)},
	{IP: net.IPv4(172, 16, 0, 0), Mask: net.CIDRMask(12, 32)},
	{IP: net.IPv4(192, 168, 0, 0), Mask: net.CIDRMask(16, 32)},
}

// dockerVMSubnets covers Docker Desktop macOS vmnet gateway IPs
var dockerVMSubnets = []net.IPNet{
	{IP: net.IPv4(172, 64, 0, 0), Mask: net.CIDRMask(13, 32)},
}

func isAllowedIP(ip net.IP) bool {
	if ip.IsLoopback() {
		return true
	}
	for _, s := range privateSubnets {
		if s.Contains(ip) {
			return true
		}
	}
	for _, s := range dockerVMSubnets {
		if s.Contains(ip) {
			return true
		}
	}
	return false
}

func Middleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// LAN access: no token needed
			ipStr, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ipStr = r.RemoteAddr
			}
			if ip := net.ParseIP(ipStr); ip != nil && isAllowedIP(ip) {
				next.ServeHTTP(w, r)
				return
			}

			// Non-LAN: require valid token
			if token == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			// Check Authorization header or query param
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				auth = auth[7:]
			}
			if auth == "" {
				auth = r.URL.Query().Get("token")
			}
			if auth != token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
