package main

import (
	"net/http"

	"github.com/boltcard/boltcard/db"
	"github.com/boltcard/boltcard/resp_err"
	log "github.com/sirupsen/logrus"
)

/**
 * @api {get} /approve Approve or reject a payment awaiting the card holder
 * @apiName ApprovePayment
 * @apiGroup BoltCardService
 *
 * @apiParam {String} token single use approval token for one payment
 * @apiParam {String} approve Y to approve the payment, N to reject it
 */

// the length of an approval token as hex
const approval_token_length = 32

// approve_request records the card holder's answer for a payment awaiting
// approval.
//
// This is a public endpoint so that the approval notification can be answered
// from a phone. It carries no API key: the token stands in for one, and a token
// is single use, unguessable, and only ever answers the one payment it was made
// for. It stops working once that payment has been sent or released.
func approve_request(w http.ResponseWriter, req *http.Request) {

	// the query string holds the token, so it is not logged

	log.WithFields(log.Fields{"path": req.URL.Path}).Debug("approve request")

	approve := req.URL.Query().Get("approve")
	if approve != "Y" && approve != "N" {
		log.Debug("approve must be Y or N")
		resp_err.Write(w)
		return
	}

	token := req.URL.Query().Get("token")
	if len(token) != approval_token_length {
		log.Debug("approval token is not the right length")
		resp_err.Write(w)
		return
	}

	answered, err := db.Approve_payment_by_token(token, approve)
	if err != nil {
		log.Warn(err)
		resp_err.Write(w)
		return
	}

	// the same answer is given for a token that is unknown, already used or
	// for a payment that has moved on, so that nothing is given away
	if !answered {
		log.Info("no payment was awaiting approval for the token given")
		resp_err.Write(w)
		return
	}

	log.WithFields(log.Fields{"approve": approve}).Info("payment approval answered")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if approve == "Y" {
		w.Write([]byte(`{"status":"OK","approved":true}`))
		return
	}

	w.Write([]byte(`{"status":"OK","approved":false}`))
}
