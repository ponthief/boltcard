package lnurlp

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/boltcard/boltcard/db"
	"github.com/boltcard/boltcard/lnd"
	"github.com/boltcard/boltcard/resp_err"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

func Callback(w http.ResponseWriter, r *http.Request) {
	if db.Get_setting("FUNCTION_LNURLP") != "ENABLE" {
		log.Debug("LNURLp function is not enabled")
		return
	}

	name := mux.Vars(r)["name"]
	amount := r.URL.Query().Get("amount")

	card_id, err := db.Get_card_id_for_name_lnurlp(name)
	if err != nil {
		log.Info("no card with lnurlp enabled for that name")
		resp_err.Write(w)
		return
	}

	log.WithFields(
		log.Fields{
			"url_path": r.URL.Path,
			"name":     name,
			"card_id":  card_id,
			"amount":   amount,
			"req.Host": r.Host,
		}).Info("lnurlp_callback")

	domain := db.Get_setting("HOST_DOMAIN")
	if r.Host != domain {
		log.Warn("wrong host domain")
		resp_err.Write(w)
		return
	}

	amount_msat, err := strconv.ParseInt(amount, 10, 64)
	if err != nil {
		log.Warn("amount is not a valid integer")
		resp_err.Write(w)
		return
	}

	// the amount must be within the range offered in the lnurlp response,
	// so that the invoice is for a whole number of satoshis and the receipt
	// records the amount the invoice was actually made out for
	if amount_msat < Min_sendable_msat || amount_msat > Max_sendable_msat {
		log.WithFields(log.Fields{"amount_msat": amount_msat}).Warn("amount is out of range")
		resp_err.Write(w)
		return
	}

	if amount_msat%1000 != 0 {
		log.WithFields(log.Fields{"amount_msat": amount_msat}).Warn("amount is not a whole number of sats")
		resp_err.Write(w)
		return
	}

	amount_sat := amount_msat / 1000

	metadata := metadata_json(name, domain)
	pr, r_hash, err := lnd.Add_invoice(amount_sat, metadata)
	if err != nil {
		log.Warn("could not add_invoice")
		resp_err.Write(w)
		return
	}

	err = db.Insert_receipt(card_id, pr, hex.EncodeToString(r_hash), amount_msat)
	if err != nil {
		log.Warn(err)
		resp_err.Write(w)
		return
	}

	go lnd.Monitor_invoice_state(r_hash)

	log.Debug("sending 'status OK' response")

	response := make(map[string]interface{})

	response["status"] = "OK"
	response["routes"] = []string{}
	response["pr"] = pr

	jsonData, err := json.Marshal(response)
	if err != nil {
		log.Warn(err)
		resp_err.Write(w)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)
}
