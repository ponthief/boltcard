package internalapi

import (
	"github.com/boltcard/boltcard/db"
	"github.com/boltcard/boltcard/resp_err"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
)

func Updateboltcard(w http.ResponseWriter, r *http.Request) {
	if db.Get_setting("FUNCTION_INTERNAL_API") != "ENABLE" {
		msg := "updateboltcard: internal API function is not enabled"
		log.Debug(msg)
		resp_err.Write_message(w, msg)
		return
	}

	enable_flag_str := r.URL.Query().Get("enable")
	enable_flag, err := strconv.ParseBool(enable_flag_str)
	if err != nil {
		msg := "updateboltcard: enable is not a valid boolean"
		log.Warn(msg)
		resp_err.Write_message(w, msg)
		return
	}

	tx_max_str := r.URL.Query().Get("tx_max")
	tx_max, err := strconv.Atoi(tx_max_str)
	if err != nil {
		msg := "updateboltcard: tx_max is not a valid integer"
		log.Warn(msg)
		resp_err.Write_message(w, msg)
		return
	}

	day_max_str := r.URL.Query().Get("day_max")
	day_max, err := strconv.Atoi(day_max_str)
	if err != nil {
		msg := "updateboltcard: day_max is not a valid integer"
		log.Warn(msg)
		resp_err.Write_message(w, msg)
		return
	}

	if !limit_valid(tx_max) || !limit_valid(day_max) {
		msg := "updateboltcard: tx_max and day_max must not be negative"
		log.Warn(msg)
		resp_err.Write_message(w, msg)
		return
	}

	card_name := r.URL.Query().Get("card_name")
	if !card_name_valid(card_name) {
		msg := "updateboltcard: the card name must be set and be at most 100 printable characters"
		log.Warn(msg)
		resp_err.Write_message(w, msg)
		return
	}

	// check if card_name exists

	card_count, err := db.Get_card_name_count(card_name)
	if err != nil {
		log.Warn(err.Error())
		resp_err.Write_message(w, "updateboltcard: the card could not be read")
		return
	}

	if card_count == 0 {
		msg := "updateboltcard: the card name does not exist in the database"
		log.Warn(msg)
		resp_err.Write_message(w, msg)
		return
	}

	// log the request

	log.WithFields(log.Fields{
		"card_name": card_name, "tx_max": tx_max, "day_max": day_max,
		"enable": enable_flag}).Info("updateboltcard API request")

	// update the card record

	err = db.Update_card(card_name, enable_flag, tx_max, day_max)
	if err != nil {
		log.Warn(err.Error())
		resp_err.Write_message(w, "updateboltcard: the card could not be updated")
		return
	}

	// send a response

	jsonData := []byte(`{"status":"OK"}`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)
}
