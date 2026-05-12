package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"share-home/internal/auth"
)

func next200() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
}

func mw(token string) http.Handler {
	return auth.Middleware(token)(next200())
}

// --- LAN bypass ---

func TestLAN_10x(t *testing.T) {
	h := mw("")
	for _, ip := range []string{"10.0.0.1", "10.255.255.255", "10.1.2.3"} {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = ip + ":12345"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Errorf("10.x LAN %s: got %d, want 200", ip, rec.Code)
		}
	}
}

func TestLAN_172x(t *testing.T) {
	h := mw("")
	for _, ip := range []string{"172.16.0.1", "172.31.255.255", "172.20.0.100"} {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = ip + ":12345"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Errorf("172.x LAN %s: got %d, want 200", ip, rec.Code)
		}
	}
}

func TestLAN_192x(t *testing.T) {
	h := mw("")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("192.168.x LAN: got %d, want 200", rec.Code)
	}
}

func TestLoopback(t *testing.T) {
	h := mw("")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("loopback: got %d, want 200", rec.Code)
	}
}

// --- Docker VM bypass (172.64.0.0/13) ---

func TestDockerVM_Bypass(t *testing.T) {
	h := mw("")
	for _, ip := range []string{"172.64.0.1", "172.64.149.246", "172.71.0.1", "172.70.255.255"} {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = ip + ":12345"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Errorf("Docker VM %s: got %d, want 200", ip, rec.Code)
		}
	}
}

// --- Public IP rejection ---

func TestPublicIP_Rejected_NoToken(t *testing.T) {
	h := mw("")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("public IP (no token): got %d, want 401", rec.Code)
	}
}

func TestPublicIP_Rejected_BadToken(t *testing.T) {
	h := mw("secret-token")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:12345"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("public IP (bad token): got %d, want 401", rec.Code)
	}
}

// --- Token auth for non-LAN ---

func TestToken_Header(t *testing.T) {
	h := mw("mytoken")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	req.Header.Set("Authorization", "Bearer mytoken")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("token header: got %d, want 200", rec.Code)
	}
}

func TestToken_HeaderNoBearer(t *testing.T) {
	h := mw("mytoken")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	req.Header.Set("Authorization", "mytoken")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("token header no bearer: got %d, want 200", rec.Code)
	}
}

func TestToken_QueryParam(t *testing.T) {
	h := mw("mytoken")
	req := httptest.NewRequest("GET", "/?token=mytoken", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("token query: got %d, want 200", rec.Code)
	}
}

func TestToken_WrongToken(t *testing.T) {
	h := mw("correct-token")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("wrong token: got %d, want 401", rec.Code)
	}
}

func TestToken_WrongQueryToken(t *testing.T) {
	h := mw("correct-token")
	req := httptest.NewRequest("GET", "/?token=wrong", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("wrong query token: got %d, want 401", rec.Code)
	}
}

// --- CF-Connecting-IP ignored (not used for auth) ---

func TestCFHeader_NotUsedForAuth(t *testing.T) {
	h := mw("")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.1:12345"
	req.Header.Set("CF-Connecting-IP", "127.0.0.1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("CF header spoof: got %d, want 401", rec.Code)
	}
}

// --- Edge cases ---

func TestBadRemoteAddr(t *testing.T) {
	h := mw("")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "invalid"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("bad RemoteAddr: got %d, want 401", rec.Code)
	}
}

func TestLAN_Bypass_IgnoresToken(t *testing.T) {
	h := mw("real-token")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.50:12345"
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("LAN with wrong token: got %d, want 200", rec.Code)
	}
}

func TestEmptyToken_DeniesNonLAN(t *testing.T) {
	h := mw("")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "45.67.89.10:12345"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("empty token non-LAN: got %d, want 401", rec.Code)
	}
}
