// Package invoice decodes lightning invoices safely.
package invoice

import (
	"errors"
	"fmt"

	decodepay "github.com/fiatjaf/ln-decodepay"
)

// Decode decodes a lightning invoice from a payment request.
//
// The underlying decoder raises a panic on some malformed input, e.g. a short
// string, so a recover turns that into an error here. The payment request
// arrives in a public request, so it must not be able to interrupt a handler
// part way through, nor stop a goroutine that is recording a payment.
func Decode(payment_request string) (bolt11 decodepay.Bolt11, err error) {

	defer func() {
		if r := recover(); r != nil {
			bolt11 = decodepay.Bolt11{}
			err = fmt.Errorf("invoice could not be decoded: %v", r)
		}
	}()

	bolt11, err = decodepay.Decodepay(payment_request)
	if err != nil {
		return decodepay.Bolt11{}, err
	}

	if bolt11.MSatoshi <= 0 {
		return decodepay.Bolt11{}, errors.New("invoice has no amount")
	}

	return bolt11, nil
}
