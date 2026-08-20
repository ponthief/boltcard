package internalapi

import (
	"github.com/boltcard/boltcard/db"
	"github.com/boltcard/boltcard/resp_err"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
)

func Updateboltcardpayment(w http.ResponseWriter, r *http.Request) {
	if db.Get_setting("FUNCTION_INTERNAL_API") != "ENABLE" {
		msg := "updateboltcard: internal API function is not enabled"
		log.Debug(msg)
		resp_err.Write_message(w, msg)
		return
	}

	ntfy_flag := r.URL.Query().Get("approve")		

	pay_id_str := r.URL.Query().Get("payment_id")
	pay_id, err := strconv.Atoi(pay_id_str)	

	// update the card payment record
    jsonData := []byte(`{"status":"OK"}`)
	err = db.Update_payment_ntfy(pay_id, ntfy_flag)
	if err != nil {
		log.Warn(err.Error())
		jsonData = []byte(err.Error())
		//return
	}

	// send a response
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)
}
