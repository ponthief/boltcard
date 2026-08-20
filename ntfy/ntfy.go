package ntfy

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	gotfy "github.com/AnthonyHewins/gotfy"
	"github.com/boltcard/boltcard/db"
	log "github.com/sirupsen/logrus"
)

type AddHeaderTransport struct {
	T http.RoundTripper
}

func (adt *AddHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", "Basic ")
	return adt.T.RoundTrip(req)
}
func NewAddHeaderTransport(T http.RoundTripper) *AddHeaderTransport {
	if T == nil {
		T = http.DefaultTransport
	}
	return &AddHeaderTransport{T}
}
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func SendNtfycation(card_payment_id int, invoice_amt int) {
	reqURLApp := fmt.Sprintf("payment_id=%s&approve=Y", strconv.Itoa(card_payment_id))
	reqURLRej := fmt.Sprintf("payment_id=%s&approve=N", strconv.Itoa(card_payment_id))
	hostName := fmt.Sprintf("%s:9001", db.Get_setting("HOST_DOMAIN"))
	message := fmt.Sprintf("Authorise %d sats payment via BoltCard", invoice_amt)
	server, _ := url.Parse("http://localhost:2586")
	customHTTPClient := http.DefaultClient
	customHTTPClient.Transport = NewAddHeaderTransport(nil)
	icon, err := url.Parse("https://boltcard.org/img/bolt-card-icon.png")
	if err != nil {
		log.Error("bad icon:" + err.Error())
	}
	tp, err := gotfy.NewPublisher(server, customHTTPClient)
	if err != nil {
		log.Error("bad config:" + err.Error())
	}
	ctx := context.Background()
	actions := []gotfy.ActionButton{&gotfy.ViewAction{
		Label: "Approve",
		Link:  &url.URL{Scheme: "http", Host: hostName, Path: "updateboltcardpayment", RawQuery: reqURLApp},
		Clear: true,
	},
		&gotfy.ViewAction{
			Label: "Reject",
			Link:  &url.URL{Scheme: "http", Host: hostName, Path: "updateboltcardpayment", RawQuery: reqURLRej},
			Clear: true,
		},
	}

	pubResp, err := tp.SendMessage(ctx, &gotfy.Message{
		Topic:    "BTC-K",
		Message:  message,
		Title:    "BoltCard Payment Request",
		Tags:     []string{"money_mouth_face"},
		Priority: gotfy.High,
		Actions:  actions,
		IconURL:  icon,
	})

	if err != nil {
		log.Error("something happened " + err.Error())
	}

	fmt.Println(pubResp)
}
