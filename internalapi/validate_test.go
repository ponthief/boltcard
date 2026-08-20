package internalapi

import (
	"strings"
	"testing"
)

func Test_card_name_valid(t *testing.T) {
	tests := []struct {
		card_name string
		want      bool
	}{
		{"card_5", true},
		{"", false},
		{strings.Repeat("a", max_card_name_length), true},
		{strings.Repeat("a", max_card_name_length+1), false},
		{"card\n5", false},
		{"card\t5", false},
	}

	for _, test := range tests {
		if got := card_name_valid(test.card_name); got != test.want {
			t.Errorf("card_name_valid(%q) = %v, want %v", test.card_name, got, test.want)
		}
	}
}

func Test_pin_valid(t *testing.T) {
	tests := []struct {
		pin_number string
		want       bool
	}{
		{"1234", true},
		{"0000", true},
		{"", false},
		{"123", false},
		{"12345", false},
		{"12a4", false},
		{"12 4", false},
	}

	for _, test := range tests {
		if got := pin_valid(test.pin_number); got != test.want {
			t.Errorf("pin_valid(%q) = %v, want %v", test.pin_number, got, test.want)
		}
	}
}

func Test_limit_valid(t *testing.T) {
	tests := []struct {
		sats int
		want bool
	}{
		{0, true},
		{1000, true},
		{-1, false},
	}

	for _, test := range tests {
		if got := limit_valid(test.sats); got != test.want {
			t.Errorf("limit_valid(%d) = %v, want %v", test.sats, got, test.want)
		}
	}
}
