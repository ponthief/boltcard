package internalapi

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/boltcard/boltcard/db"
	"github.com/boltcard/boltcard/resp_err"
	log "github.com/sirupsen/logrus"
)

func random_hex() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		log.Warn(err.Error())
		return ""
	}

	return hex.EncodeToString(b)
}

func Createboltcardwithpin(w http.ResponseWriter, r *http.Request) {
	if db.Get_setting("FUNCTION_INTERNAL_API") != "ENABLE" {
		msg := "createboltcardwithpin: internal API function is not enabled"
		log.Debug(msg)
		resp_err.Write_message(w, msg)
		return
	}

	tx_max_str := r.URL.Query().Get("tx_max")
	tx_max, err := strconv.Atoi(tx_max_str)
	if err != nil {
		msg := "createboltcardwithpin: tx_max is not a valid integer"
		log.Warn(msg)
		resp_err.Write_message(w, msg)
		return
	}

	day_max_str := r.URL.Query().Get("day_max")
	day_max, err := strconv.Atoi(day_max_str)
	if err != nil {
		msg := "createboltcardwithpin: day_max is not a valid integer"
		log.Warn(msg)
		resp_err.Write_message(w, msg)
		return
	}

	enable_flag_str := r.URL.Query().Get("enable")
	enable_flag, err := strconv.ParseBool(enable_flag_str)
	if err != nil {
		msg := "createboltcardwithpin: enable is not a valid boolean"
		log.Warn(msg)
		resp_err.Write_message(w, msg)
		return
	}

	if !limit_valid(tx_max) || !limit_valid(day_max) {
		msg := "createboltcardwithpin: tx_max and day_max must not be negative"
		log.Warn(msg)
		resp_err.Write_message(w, msg)
		return
	}

	card_name := r.URL.Query().Get("card_name")
	if !card_name_valid(card_name) {
		msg := "createboltcardwithpin: the card name must be set and be at most 100 printable characters"
		log.Warn(msg)
		resp_err.Write_message(w, msg)
		return
	}

	uid_privacy_flag_str := r.URL.Query().Get("uid_privacy")
	uid_privacy_flag, err := strconv.ParseBool(uid_privacy_flag_str)
	if err != nil {
		msg := "createboltcardwithpin: uid_privacy is not a valid boolean"
		log.Warn(msg)
		resp_err.Write_message(w, msg)
		return
	}

	allow_neg_bal_flag_str := r.URL.Query().Get("allow_neg_bal")
	allow_neg_bal_flag, err := strconv.ParseBool(allow_neg_bal_flag_str)
	if err != nil {
		msg := "createboltcardwithpin: allow_neg_bal is not a valid boolean"
		log.Warn(msg)
		resp_err.Write_message(w, msg)
		return
	}

	pin_enable_flag_str := r.URL.Query().Get("enable_pin")
	pin_enable_flag, err := strconv.ParseBool(pin_enable_flag_str)
	if err != nil {
		msg := "createboltcardwithpin: enable_pin is not a valid boolean"
		log.Warn(msg)
		resp_err.Write_message(w, msg)
		return
	}

	pin_number := r.URL.Query().Get("pin_number")
	if !pin_valid(pin_number) {
		msg := "createboltcardwithpin: pin_number must be four digits"
		log.Warn(msg)
		resp_err.Write_message(w, msg)
		return
	}

	pin_limit_sats_str := r.URL.Query().Get("pin_limit_sats")
	pin_limit_sats, err := strconv.Atoi(pin_limit_sats_str)
	if err != nil {
		msg := "createboltcardwithpin: pin_limit_sats is not a valid integer"
		log.Warn(msg)
		resp_err.Write_message(w, msg)
		return
	}

	if !limit_valid(pin_limit_sats) {
		msg := "createboltcardwithpin: pin_limit_sats must not be negative"
		log.Warn(msg)
		resp_err.Write_message(w, msg)
		return
	}

	// log the request
	// the PIN is a secret, so it is not logged

	log.WithFields(log.Fields{
		"card_name": card_name, "tx_max": tx_max, "day_max": day_max,
		"enable": enable_flag, "uid_privacy": uid_privacy_flag,
		"allow_neg_bal": allow_neg_bal_flag, "enable_pin": pin_enable_flag,
		"pin_limit_sats": pin_limit_sats}).Info("createboltcardwithpin API request")

	// create the keys

	one_time_code := random_hex()
	k0_auth_key := random_hex()
	k2_cmac_key := random_hex()
	k3 := random_hex()
	k4 := random_hex()

	// create the new card record

	err = db.Insert_card_with_pin(one_time_code, k0_auth_key, k2_cmac_key, k3, k4,
		tx_max, day_max, enable_flag, card_name,
		uid_privacy_flag, allow_neg_bal_flag, pin_enable_flag, pin_number, pin_limit_sats)
	if err != nil {
		log.Warn(err.Error())
		resp_err.Write_message(w, "createboltcardwithpin: the card could not be created")
		return
	}

	// return the URI + one_time_code

	hostdomain := db.Get_setting("HOST_DOMAIN")
	url := ""
	if strings.HasSuffix(hostdomain, ".onion") {
		url = "http://" + hostdomain + "/new?a=" + one_time_code
	} else {
		url = "https://" + hostdomain + "/new?a=" + one_time_code
	}

	// log the response
	// the URL holds the one time code, which is a secret, so it is not logged

	log.WithFields(log.Fields{
		"card_name": card_name}).Info("createboltcardwithpin API response")

	jsonData := []byte(`{"status":"OK",` +
		`"url":"` + url + `"}`)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)
}
