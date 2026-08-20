package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func Test_allow_up_to_burst(t *testing.T) {
	l := New(60, 3)

	for i := 0; i < 3; i++ {
		if !l.Allow("10.0.0.1") {
			t.Fatalf("request %d was rejected within the burst", i+1)
		}
	}

	if l.Allow("10.0.0.1") {
		t.Error("a request over the burst was allowed")
	}
}

func Test_clients_are_limited_separately(t *testing.T) {
	l := New(60, 1)

	if !l.Allow("10.0.0.1") {
		t.Fatal("the first request from the first client was rejected")
	}

	if l.Allow("10.0.0.1") {
		t.Error("a second request from the first client was allowed")
	}

	if !l.Allow("10.0.0.2") {
		t.Error("the first request from a second client was rejected")
	}
}

func Test_bucket_refills_over_time(t *testing.T) {
	now := time.Now()

	l := New(60, 1)
	l.now = func() time.Time { return now }

	if !l.Allow("10.0.0.1") {
		t.Fatal("the first request was rejected")
	}

	if l.Allow("10.0.0.1") {
		t.Fatal("a second request in the same instant was allowed")
	}

	// 60 requests per minute refills one token per second
	now = now.Add(2 * time.Second)

	if !l.Allow("10.0.0.1") {
		t.Error("a request after the bucket refilled was rejected")
	}
}

func Test_idle_buckets_are_pruned(t *testing.T) {
	now := time.Now()

	l := New(60, 1)
	l.now = func() time.Time { return now }

	l.Allow("10.0.0.1")

	now = now.Add(bucket_expiry + time.Minute)

	l.Allow("10.0.0.2")

	l.mutex.Lock()
	bucket_count := len(l.buckets)
	l.mutex.Unlock()

	if bucket_count != 1 {
		t.Errorf("bucket count = %d, want 1 after pruning", bucket_count)
	}
}

func Test_middleware_rejects_over_the_limit(t *testing.T) {
	l := New(60, 1)

	calls := 0
	handler := l.Middleware(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	})

	first := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/createboltcard", nil)
	request.RemoteAddr = "10.0.0.1:54321"
	handler(first, request)

	if first.Code != http.StatusOK {
		t.Errorf("first request status = %d, want %d", first.Code, http.StatusOK)
	}

	second := httptest.NewRecorder()
	handler(second, request)

	if second.Code != http.StatusTooManyRequests {
		t.Errorf("second request status = %d, want %d", second.Code, http.StatusTooManyRequests)
	}

	if calls != 1 {
		t.Errorf("handler calls = %d, want 1", calls)
	}
}

func Test_client_key(t *testing.T) {
	tests := []struct {
		remote_addr string
		want        string
	}{
		{"10.0.0.1:54321", "10.0.0.1"},
		{"[::1]:9001", "::1"},
		{"no-port", "no-port"},
	}

	for _, test := range tests {
		r := httptest.NewRequest("GET", "/ping", nil)
		r.RemoteAddr = test.remote_addr

		if got := Client_key(r); got != test.want {
			t.Errorf("Client_key(%q) = %q, want %q", test.remote_addr, got, test.want)
		}
	}
}

func Test_client_key_behind_proxies(t *testing.T) {
	tests := []struct {
		name                string
		remote_addr         string
		forwarded_for       string
		trusted_proxy_count int
		want                string
	}{
		{
			name:                "no proxy trusted, header ignored",
			remote_addr:         "10.0.0.1:54321",
			forwarded_for:       "203.0.113.9",
			trusted_proxy_count: 0,
			want:                "10.0.0.1",
		},
		{
			name:                "one proxy, caller taken from the header",
			remote_addr:         "10.0.0.1:54321",
			forwarded_for:       "203.0.113.9",
			trusted_proxy_count: 1,
			want:                "203.0.113.9",
		},
		{
			name:                "one proxy, a caller supplied entry is not trusted",
			remote_addr:         "10.0.0.1:54321",
			forwarded_for:       "198.51.100.7, 203.0.113.9",
			trusted_proxy_count: 1,
			want:                "203.0.113.9",
		},
		{
			name:                "two proxies",
			remote_addr:         "10.0.0.1:54321",
			forwarded_for:       "203.0.113.9, 10.0.0.2",
			trusted_proxy_count: 2,
			want:                "203.0.113.9",
		},
		{
			name:                "two proxies, spoofed entries ahead of the caller",
			remote_addr:         "10.0.0.1:54321",
			forwarded_for:       "1.1.1.1, 2.2.2.2, 203.0.113.9, 10.0.0.2",
			trusted_proxy_count: 2,
			want:                "203.0.113.9",
		},
		{
			name:                "fewer entries than proxies configured",
			remote_addr:         "10.0.0.1:54321",
			forwarded_for:       "203.0.113.9",
			trusted_proxy_count: 3,
			want:                "203.0.113.9",
		},
		{
			name:                "no header from the proxy",
			remote_addr:         "10.0.0.1:54321",
			forwarded_for:       "",
			trusted_proxy_count: 1,
			want:                "10.0.0.1",
		},
		{
			name:                "header value is not an address",
			remote_addr:         "10.0.0.1:54321",
			forwarded_for:       "not-an-address",
			trusted_proxy_count: 1,
			want:                "10.0.0.1",
		},
		{
			name:                "ipv6 caller",
			remote_addr:         "10.0.0.1:54321",
			forwarded_for:       "2001:db8::1",
			trusted_proxy_count: 1,
			want:                "2001:db8::1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/ln", nil)
			r.RemoteAddr = test.remote_addr
			if test.forwarded_for != "" {
				r.Header.Set("X-Forwarded-For", test.forwarded_for)
			}

			if got := Client_key_behind_proxies(r, test.trusted_proxy_count); got != test.want {
				t.Errorf("Client_key_behind_proxies() = %q, want %q", got, test.want)
			}
		})
	}
}

// a caller must not be able to dodge the limiter by varying a header
func Test_spoofed_forwarded_header_cannot_dodge_the_limiter(t *testing.T) {
	l := New(60, 2)
	l.Key_func = Key_func_for_proxies(0)

	handler := l.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	statuses := []int{}
	for i := 0; i < 4; i++ {
		r := httptest.NewRequest("GET", "/ln", nil)
		r.RemoteAddr = "10.0.0.1:54321"
		r.Header.Set("X-Forwarded-For", "203.0.113."+string(rune('1'+i)))
		w := httptest.NewRecorder()
		handler(w, r)
		statuses = append(statuses, w.Code)
	}

	if statuses[2] != http.StatusTooManyRequests || statuses[3] != http.StatusTooManyRequests {
		t.Errorf("statuses = %v, want the last two to be %d", statuses, http.StatusTooManyRequests)
	}
}
