package internalapi

import (
	"net/http"
	"strconv"

	"github.com/boltcard/boltcard/db"
	"github.com/boltcard/boltcard/resp_err"
	log "github.com/sirupsen/logrus"
)

func Updateboltcardpayment(w http.ResponseWriter, r *http.Request) {
	if db.Get_setting("FUNCTION_INTERNAL_API") != "ENABLE" {
		msg := "updateboltcardpayment: internal API function is not enabled"
		log.Debug(msg)
		resp_err.Write_message(w, msg)
		return
	}

	ntfy_flag := r.URL.Query().Get("approve")
	if ntfy_flag != "Y" && ntfy_flag != "N" {
		msg := "updateboltcardpayment: approve must be Y or N"
		log.Warn(msg)
		resp_err.Write_message(w, msg)
		return
	}

	pay_id_str := r.URL.Query().Get("payment_id")
	pay_id, err := strconv.Atoi(pay_id_str)
	if err != nil {
		msg := "updateboltcardpayment: payment_id is not a valid integer"
		log.Warn(msg)
		resp_err.Write_message(w, msg)
		return
	}

	// log the request

	log.WithFields(log.Fields{
		"card_payment_id": pay_id, "approve": ntfy_flag}).Info("updateboltcardpayment API request")

	// update the card payment record

	err = db.Update_payment_ntfy(pay_id, ntfy_flag)
	if err != nil {
		log.Warn(err.Error())
		resp_err.Write_message(w, "updateboltcardpayment: the card payment could not be updated")
		return
	}

	// send a response

	jsonData := []byte(`{"status":"OK"}`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)
}
