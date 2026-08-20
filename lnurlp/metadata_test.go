package lnurlp

import (
	"encoding/json"
	"testing"
)

func Test_metadata_json(t *testing.T) {
	got := metadata_json("card_5", "card.example.com")
	want := `[["text/identifier","card_5@card.example.com"],["text/plain","bolt card deposit"]]`

	if got != want {
		t.Errorf("metadata_json() = %s, want %s", got, want)
	}
}

// the metadata is served in the lnurlp response and hashed into the invoice by
// the callback, so it has to stay valid JSON whatever the card is named
func Test_metadata_json_escapes_a_quoted_name(t *testing.T) {
	got := metadata_json(`a"b\c`, "card.example.com")

	var decoded [][]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("metadata_json() produced invalid JSON: %s (%v)", got, err)
	}

	if decoded[0][1] != `a"b\c@card.example.com` {
		t.Errorf("identifier = %s, want %s", decoded[0][1], `a"b\c@card.example.com`)
	}
}

func Test_sendable_range(t *testing.T) {
	// the callback enforces this range, so it must be a whole number of sats
	if Min_sendable_msat%1000 != 0 || Max_sendable_msat%1000 != 0 {
		t.Error("the sendable range must be a whole number of sats")
	}

	if Min_sendable_msat < 1000 {
		t.Error("the minimum sendable amount must be at least one sat")
	}

	if Max_sendable_msat <= Min_sendable_msat {
		t.Error("the maximum sendable amount must be above the minimum")
	}
}
