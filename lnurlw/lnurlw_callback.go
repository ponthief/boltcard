package lnurlw

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/boltcard/boltcard/db"
	"github.com/boltcard/boltcard/invoice"
	"github.com/boltcard/boltcard/lnd"
	"github.com/boltcard/boltcard/lndhub"
	ntfy "github.com/boltcard/boltcard/ntfy"
	"github.com/boltcard/boltcard/resp_err"
	"github.com/boltcard/boltcard/safego"
	decodepay "github.com/fiatjaf/ln-decodepay"
	log "github.com/sirupsen/logrus"
)

// how long the card holder has to approve a payment, and how often the answer
// is looked for
const approval_period = 1 * time.Minute
const approval_poll_interval = 2 * time.Second

type LndhubAuthRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type LndhubAuthResponse struct {
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
}

func lndhub_payment(w http.ResponseWriter, p *db.Payment, bolt11 decodepay.Bolt11, param_pr string) {

	//get setting for LNDHUB_URL
	lndhub_url := db.Get_setting("LNDHUB_URL")

	//get lndhub login details from database
	c, err := db.Get_card_from_card_id(p.Card_id)
	if err != nil {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Warn(err)
		resp_err.Write(w)
		return
	}

	invoice_sats := int(bolt11.MSatoshi / 1000)

	//lndhub.auth API call
	//the login JSON is held in the Card_name field
	// as "login:password"
	card_name_parts := strings.Split(c.Card_name, ":")

	if len(card_name_parts) != 2 {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Warn("login:password not found")
		resp_err.Write(w)
		return
	}

	if len(card_name_parts[0]) != 20 || len(card_name_parts[1]) != 20 {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Warn("login:password badly formed")
		resp_err.Write(w)
		return
	}

	var lhAuthRequest LndhubAuthRequest
	lhAuthRequest.Login = card_name_parts[0]
	lhAuthRequest.Password = card_name_parts[1]

	authReq, err := json.Marshal(lhAuthRequest)

	req_auth, err := http.NewRequest("POST", lndhub_url+"/auth", bytes.NewBuffer(authReq))
	if err != nil {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Warn(err)
		resp_err.Write(w)
		return
	}

	req_auth.Header.Add("Access-Control-Allow-Origin", "*")
	req_auth.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp_auth, err := client.Do(req_auth)
	if err != nil {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Warn(err)
		resp_err.Write(w)
		return
	}

	defer resp_auth.Body.Close()

	resp_auth_bytes, err := io.ReadAll(resp_auth.Body)
	if err != nil {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Warn(err)
		resp_err.Write(w)
		return
	}

	// the response body holds the lndhub access and refresh tokens,
	// which are secrets, so it is not logged

	log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id,
		"status": resp_auth.StatusCode}).Debug("lndhub auth response")

	var auth_keys LndhubAuthResponse

	err = json.Unmarshal([]byte(resp_auth_bytes), &auth_keys)
	if err != nil {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Warn(err)
		resp_err.Write(w)
		return
	}

	// check the payment rules and claim the payment, so that concurrent
	// callbacks for one withdraw request cannot each send a payment
	// lndhub payments are limited per transaction only, as before
	authorized, refusal, err := db.Authorize_payment(p.Card_payment_id, invoice_sats, false)
	if err != nil {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Warn(err)
		resp_err.Write(w)
		return
	}

	if !authorized {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id,
			"invoice_sats": invoice_sats}).Info("payment not authorized - ", refusal)
		resp_err.Write(w)
		return
	}

	// https://github.com/fiatjaf/lnurl-rfc/blob/luds/03.md
	//
	// LN SERVICE sends a {"status": "OK"} or
	// {"status": "ERROR", "reason": "error details..."}
	//  JSON response and then attempts to pay the invoices asynchronously.

	go lndhub.PayInvoice(p.Card_payment_id, param_pr, invoice_sats, card_name_parts[0], auth_keys.AccessToken)

	log.Debug("sending 'status OK' response")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jsonData := []byte(`{"status":"OK"}`)
	w.Write(jsonData)
}

func lnd_payment(w http.ResponseWriter, p *db.Payment, bolt11 decodepay.Bolt11, param_pr string) {

	invoice_sats := int(bolt11.MSatoshi / 1000)

	// check the transaction limit, the daily limit and the card balance, and
	// claim the payment, as one atomic step
	//
	// doing this in one step is what stops concurrent callbacks for a single
	// withdraw request each sending a payment, and stops parallel card taps
	// each being measured against the same daily total or balance
	authorized, refusal, err := db.Authorize_payment(p.Card_payment_id, invoice_sats, true)
	if err != nil {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Warn(err)
		resp_err.Write(w)
		return
	}

	if !authorized {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id,
			"invoice_sats": invoice_sats}).Info("payment not authorized - ", refusal)
		resp_err.Write(w)
		return
	}

	// the payment was claimed by Authorize_payment above, so it is not
	// claimed again here

	// ask the card holder to approve the payment, where that is switched on
	//
	// https://github.com/fiatjaf/lnurl-rfc/blob/luds/03.md
	//
	// LN SERVICE sends a {"status": "OK"} or
	// {"status": "ERROR", "reason": "error details..."}
	//  JSON response and then attempts to pay the invoices asynchronously.
	//
	// so the response is sent now and the waiting is done in its own
	// goroutine - waiting here would hold the request open past the write
	// timeout of the server and would hold one of the places for handling
	// requests at once for the whole approval period

	if db.Get_setting("FUNCTION_NTFY") == "ENABLE" {
		go await_approval(p.Card_payment_id, invoice_sats, param_pr)
	} else {
		go lnd.PayInvoice(p.Card_payment_id, param_pr)
	}

	log.Debug("sending 'status OK' response")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jsonData := []byte(`{"status":"OK"}`)
	w.Write(jsonData)
}

// await_approval asks the card holder to approve a claimed payment and pays it
// once they do.
//
// A payment that is rejected, or that is not answered within the approval
// period, is released so that it counts towards neither the daily limit nor the
// card balance. The claim stays in place either way, so the card tap is spent
// and the withdraw request cannot be used again.
func await_approval(card_payment_id int, invoice_sats int, param_pr string) {

	defer safego.Recover("await_approval")

	// the notification carries a single use token rather than an API key
	token, err := db.Create_payment_ntfy_token(card_payment_id)
	if err != nil {
		log.WithFields(log.Fields{"card_payment_id": card_payment_id}).Warn(err)
		release(card_payment_id, "approval could not be requested")
		return
	}

	err = ntfy.SendNtfycation(card_payment_id, invoice_sats, token)
	if err != nil {
		log.WithFields(log.Fields{"card_payment_id": card_payment_id}).Warn(err)
		release(card_payment_id, "approval could not be requested")
		return
	}

	deadline := time.Now().Add(approval_period)

	for time.Now().Before(deadline) {
		time.Sleep(approval_poll_interval)

		answered, approved, err := db.Get_payment_approval(card_payment_id)
		if err != nil {
			log.WithFields(log.Fields{"card_payment_id": card_payment_id}).Warn(err)
			continue
		}

		if !answered {
			continue
		}

		if approved {
			log.WithFields(log.Fields{"card_payment_id": card_payment_id}).Info("payment approved")
			lnd.PayInvoice(card_payment_id, param_pr)
			return
		}

		log.WithFields(log.Fields{"card_payment_id": card_payment_id}).Info("payment rejected")
		release(card_payment_id, "payment was rejected")
		return
	}

	log.WithFields(log.Fields{"card_payment_id": card_payment_id}).Info("payment approval timed out")
	release(card_payment_id, "payment was not approved in time")
}

// release marks a claimed payment that will not be sent as failed
func release(card_payment_id int, reason string) {
	err := db.Release_payment(card_payment_id, reason)
	if err != nil {
		log.WithFields(log.Fields{"card_payment_id": card_payment_id}).Warn(err)
	}
}

func Callback(w http.ResponseWriter, req *http.Request) {

	env_host_domain := db.Get_setting("HOST_DOMAIN")
	if req.Host != env_host_domain {
		log.Warn("wrong host domain")
		resp_err.Write(w)
		return
	}

	// the query string holds k1 and any PIN, so it is not logged

	log.WithFields(log.Fields{"path": req.URL.Path}).Debug("cb request")

	// get k1 value
	param_k1 := req.URL.Query().Get("k1")

	if param_k1 == "" {
		log.Debug("k1 not found")
		resp_err.Write(w)
		return
	}

	p, err := db.Get_payment_k1(param_k1)
	if err != nil {
		log.Warn("no withdraw request found for the k1 given: ", err)
		resp_err.Write(w)
		return
	}

	// check that payment has not been made
	if p.Paid_flag != "N" {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Info("payment already made")
		resp_err.Write(w)
		return
	}

	// check if lnurlw_request has timed out
	lnurlw_timeout, err := db.Check_lnurlw_timeout(p.Card_payment_id)
	if err != nil {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Warn(err)
		resp_err.Write(w)
		return
	}
	if lnurlw_timeout == true {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Info("lnurlw request has timed out")
		resp_err.Write(w)
		return
	}

	// get the payment request
	param_pr := req.URL.Query().Get("pr")
	if param_pr == "" {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Warn("pr field not found")
		resp_err.Write(w)
		return
	}

	bolt11, err := invoice.Decode(param_pr)
	if err != nil {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Warn(err)
		resp_err.Write(w)
		return
	}

	// record the lightning invoice
	err = db.Update_payment_invoice(p.Card_payment_id, param_pr, bolt11.MSatoshi)
	if err != nil {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Warn(err)
		resp_err.Write(w)
		return
	}

	log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Debug("checking payment rules")

	// get the pin if it has been passed in
	param_pin := req.URL.Query().Get("pin")

	c, err := db.Get_card_from_card_id(p.Card_id)
	if err != nil {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Warn(err)
		resp_err.Write(w)
		return
	}

	// check the pin if needed
	//
	// a wrong PIN consumes the withdraw request, so that a PIN can only be
	// tried once per card tap rather than being guessed in bulk
	// the comparison is constant time so that it gives nothing away
	if c.Pin_enable == "Y" && int(bolt11.MSatoshi/1000) >= c.Pin_limit_sats &&
		subtle.ConstantTimeCompare([]byte(c.Pin_number), []byte(param_pin)) != 1 {

		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Warn("incorrect pin provided")

		err = db.Fail_payment(p.Card_payment_id, "incorrect pin provided")
		if err != nil {
			log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Warn(err)
		}

		resp_err.Write(w)
		return
	}

	// check if we are only sending funds to a defined test node
	testnode := db.Get_setting("LN_TESTNODE")
	if testnode != "" && bolt11.Payee != testnode {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Info("rejected as not the defined test node")
		resp_err.Write(w)
		return
	}

	//check if we are using LND or LNDHUB for payment
	lndhub := db.Get_setting("FUNCTION_LNDHUB")
	if lndhub == "ENABLE" {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Info("initiating lndhub payment")
		lndhub_payment(w, p, bolt11, param_pr)
	} else {
		log.WithFields(log.Fields{"card_payment_id": p.Card_payment_id}).Info("initiating lnd payment")
		lnd_payment(w, p, bolt11, param_pr)
	}
}
