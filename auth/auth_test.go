package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const test_key = "0123456789abcdef0123456789abcdef"

func Test_token_valid(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		presented  string
		want       bool
	}{
		{"matching key", test_key, test_key, true},
		{"wrong key", test_key, strings.Repeat("f", len(test_key)), false},
		{"no key presented", test_key, "", false},
		{"no key configured", "", test_key, false},
		{"no key configured or presented", "", "", false},
		{"configured key too short", "short", "short", false},
		{"presented key is a prefix", test_key, test_key[:16], false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Token_valid(test.configured, test.presented)
			if got != test.want {
				t.Errorf("Token_valid(%q, %q) = %v, want %v",
					test.configured, test.presented, got, test.want)
			}
		})
	}
}

func Test_request_token(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{"bearer", map[string]string{"Authorization": "Bearer " + test_key}, test_key},
		{"bearer lower case scheme", map[string]string{"Authorization": "bearer " + test_key}, test_key},
		{"bearer with spaces", map[string]string{"Authorization": "Bearer  " + test_key + " "}, test_key},
		{"api key header", map[string]string{"X-Api-Key": test_key}, test_key},
		{"other scheme ignored", map[string]string{"Authorization": "Basic " + test_key}, ""},
		{"no headers", map[string]string{}, ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/createboltcard", nil)
			for name, value := range test.headers {
				r.Header.Set(name, value)
			}

			got := Request_token(r)
			if got != test.want {
				t.Errorf("Request_token() = %q, want %q", got, test.want)
			}
		})
	}
}

func Test_require_internal_api_key(t *testing.T) {
	tests := []struct {
		name            string
		configured_key  string
		auth_header     string
		want_status     int
		want_handler_ok bool
	}{
		{"valid key", test_key, "Bearer " + test_key, http.StatusOK, true},
		{"wrong key", test_key, "Bearer " + strings.Repeat("f", len(test_key)), http.StatusUnauthorized, false},
		{"missing header", test_key, "", http.StatusUnauthorized, false},
		{"no key configured", "", "Bearer " + test_key, http.StatusUnauthorized, false},
		{"no key configured and none presented", "", "", http.StatusUnauthorized, false},
		{"weak key configured", "tooshort", "Bearer tooshort", http.StatusUnauthorized, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original_provider := Key_provider
			Key_provider = func() string { return test.configured_key }
			defer func() { Key_provider = original_provider }()

			handler_called := false
			handler := Require_internal_api_key(func(w http.ResponseWriter, r *http.Request) {
				handler_called = true
				w.WriteHeader(http.StatusOK)
			})

			r := httptest.NewRequest("GET", "/createboltcard?card_name=card_1", nil)
			if test.auth_header != "" {
				r.Header.Set("Authorization", test.auth_header)
			}
			w := httptest.NewRecorder()

			handler(w, r)

			if w.Code != test.want_status {
				t.Errorf("status = %d, want %d", w.Code, test.want_status)
			}

			if handler_called != test.want_handler_ok {
				t.Errorf("handler called = %v, want %v", handler_called, test.want_handler_ok)
			}
		})
	}
}

func Test_key_configured(t *testing.T) {
	original_provider := Key_provider
	defer func() { Key_provider = original_provider }()

	Key_provider = func() string { return "" }
	if Key_configured() {
		t.Error("Key_configured() = true with no key set")
	}

	Key_provider = func() string { return "tooshort" }
	if Key_configured() {
		t.Error("Key_configured() = true with a key shorter than the minimum")
	}

	Key_provider = func() string { return test_key }
	if !Key_configured() {
		t.Error("Key_configured() = false with a valid key set")
	}
}

func Test_internal_api_key_from_environment(t *testing.T) {
	t.Setenv("INTERNAL_API_KEY", "  "+test_key+"  ")

	if got := Internal_api_key(); got != test_key {
		t.Errorf("Internal_api_key() = %q, want %q", got, test_key)
	}
}
