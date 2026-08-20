package internalapi

import (
	"strings"
	"unicode"
)

// the card_name column is VARCHAR(100)
const max_card_name_length = 100

// the pin_number column is CHAR(4)
const pin_length = 4

// card_name_valid checks a card name is present, fits the database column and
// holds no control characters, which would end up in log entries.
func card_name_valid(card_name string) bool {
	if card_name == "" || len(card_name) > max_card_name_length {
		return false
	}

	for _, r := range card_name {
		if !unicode.IsPrint(r) {
			return false
		}
	}

	return true
}

// pin_valid checks a PIN is exactly four decimal digits.
func pin_valid(pin_number string) bool {
	if len(pin_number) != pin_length {
		return false
	}

	return strings.IndexFunc(pin_number, func(r rune) bool {
		return r < '0' || r > '9'
	}) == -1
}

// limit_valid checks a satoshi limit is not negative.
func limit_valid(sats int) bool {
	return sats >= 0
}
