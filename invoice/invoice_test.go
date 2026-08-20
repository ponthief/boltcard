package invoice

import (
	"strings"
	"testing"
)

// a bolt11 invoice for 250000 sats
const valid_invoice = "lnbc2500u1pvjluezpp5qqqsyqcyq5rqwzqfqqqsyqcyq5rqwzqfqqqsyqcyq5rqwzqf" +
	"qypqdq5xysxxatsyp3k7enxv4jsxqzpuaztrnwngzn3kdzw5hydlzf03qdgm2hdq27cqv3agm2awhz5se903v" +
	"ruatfhq77w3ls4evs3ch9zw97j25emudupq63nyw24cg27h2rspfj9srp"

func Test_decode_a_valid_invoice(t *testing.T) {
	bolt11, err := Decode(valid_invoice)
	if err != nil {
		t.Fatalf("Decode errored on a valid invoice: %v", err)
	}

	if bolt11.MSatoshi != 250000000 {
		t.Errorf("MSatoshi = %d, want 250000000", bolt11.MSatoshi)
	}
}

// the decoder panics on some malformed input, which must not reach the caller
func Test_decode_returns_an_error_and_never_panics(t *testing.T) {
	tests := []struct {
		name            string
		payment_request string
	}{
		{"empty", ""},
		{"short rubbish", "notaninvoice"},
		{"one character", "l"},
		{"prefix only", "lnbc"},
		{"prefix and amount", "lnbc2500u"},
		{"wrong checksum", strings.Replace(valid_invoice, "fj9srp", "fj9srq", 1)},
		{"truncated invoice", valid_invoice[:40]},
		{"not bech32", "!!!!!!!!!!!!"},
		{"very long rubbish", strings.Repeat("x", 2000)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// a panic escaping Decode fails the test by crashing it
			bolt11, err := Decode(test.payment_request)

			if err == nil {
				t.Errorf("Decode(%q) returned no error", test.payment_request)
			}

			if bolt11.MSatoshi != 0 {
				t.Errorf("Decode(%q) returned an amount of %d", test.payment_request, bolt11.MSatoshi)
			}
		})
	}
}
