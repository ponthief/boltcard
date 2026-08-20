package lnurlp

import (
	"encoding/json"
)

// the range of amounts offered in the lnurlp response and enforced in the
// lnurlp callback
const Min_sendable_msat = 1000
const Max_sendable_msat = 1000000000

// metadata_json returns the lnurlp metadata for a lightning address.
//
// The same text is served in the lnurlp response and hashed into the invoice
// description hash by the callback, so both must build it here. Values are
// JSON encoded, so that a name holding a quote cannot break the response.
func metadata_json(name string, domain string) string {
	metadata := [][]string{
		{"text/identifier", name + "@" + domain},
		{"text/plain", "bolt card deposit"},
	}

	encoded, err := json.Marshal(metadata)
	if err != nil {
		return ""
	}

	return string(encoded)
}
