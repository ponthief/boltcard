package ntfy

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	gotfy "github.com/AnthonyHewins/gotfy"
	"github.com/boltcard/boltcard/db"
	log "github.com/sirupsen/logrus"
)

// the ntfy server to publish to, when NTFY_URL is not set
const default_ntfy_url = "http://localhost:2586"

type AddHeaderTransport struct {
	T    http.RoundTripper
	Auth string
}

func (adt *AddHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if adt.Auth != "" {
		req.Header.Add("Authorization", "Basic "+adt.Auth)
	}
	return adt.T.RoundTrip(req)
}

func NewAddHeaderTransport(T http.RoundTripper, auth string) *AddHeaderTransport {
	if T == nil {
		T = http.DefaultTransport
	}
	return &AddHeaderTransport{T: T, Auth: auth}
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

// SendNtfycation asks the card holder to approve a payment.
//
// The approve and reject links carry a single use token for this payment and
// point at the public API, so that no API key has to travel to the ntfy server
// or sit in a notification on a phone.
func SendNtfycation(card_payment_id int, invoice_amt int, approval_token string) error {

	host_domain := db.Get_setting("HOST_DOMAIN")
	if host_domain == "" {
		return errors.New("HOST_DOMAIN is not set")
	}

	topic := db.Get_setting("NTFY_TOPIC")
	if topic == "" {
		return errors.New("NTFY_TOPIC is not set")
	}

	ntfy_url := db.Get_setting("NTFY_URL")
	if ntfy_url == "" {
		ntfy_url = default_ntfy_url
	}

	server, err := url.Parse(ntfy_url)
	if err != nil {
		return fmt.Errorf("NTFY_URL is not a valid URL: %w", err)
	}

	// a client of its own, so that the header this adds is not put on every
	// other request the service makes
	auth := ""
	ntfy_user := db.Get_setting("NTFY_USER")
	if ntfy_user != "" {
		auth = basicAuth(ntfy_user, db.Get_setting("NTFY_PASSWORD"))
	}

	client := &http.Client{Transport: NewAddHeaderTransport(nil, auth)}

	publisher, err := gotfy.NewPublisher(server, client)
	if err != nil {
		return fmt.Errorf("could not reach the ntfy server: %w", err)
	}

	scheme := "https"
	if strings.HasSuffix(host_domain, ".onion") {
		scheme = "http"
	}

	approve_link := &url.URL{
		Scheme:   scheme,
		Host:     host_domain,
		Path:     "/approve",
		RawQuery: "token=" + url.QueryEscape(approval_token) + "&approve=Y",
	}

	reject_link := &url.URL{
		Scheme:   scheme,
		Host:     host_domain,
		Path:     "/approve",
		RawQuery: "token=" + url.QueryEscape(approval_token) + "&approve=N",
	}

	actions := []gotfy.ActionButton{
		&gotfy.ViewAction{
			Label: "Approve",
			Link:  approve_link,
			Clear: true,
		},
		&gotfy.ViewAction{
			Label: "Reject",
			Link:  reject_link,
			Clear: true,
		},
	}

	icon, err := url.Parse("https://boltcard.org/img/bolt-card-icon.png")
	if err != nil {
		log.Warn("bad icon: " + err.Error())
	}

	_, err = publisher.SendMessage(context.Background(), &gotfy.Message{
		Topic:    topic,
		Message:  fmt.Sprintf("Authorise %d sats payment via BoltCard", invoice_amt),
		Title:    "BoltCard Payment Request",
		Tags:     []string{"money_mouth_face"},
		Priority: gotfy.High,
		Actions:  actions,
		IconURL:  icon,
	})
	if err != nil {
		return fmt.Errorf("could not send the approval notification: %w", err)
	}

	// the token is a secret, so the links are not logged
	log.WithFields(log.Fields{
		"card_payment_id": card_payment_id}).Info("payment approval requested")

	return nil
}
