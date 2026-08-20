package main

import (
	"github.com/boltcard/boltcard/auth"
	"github.com/boltcard/boltcard/db"
	"github.com/boltcard/boltcard/internalapi"
	"github.com/boltcard/boltcard/lnurlp"
	"github.com/boltcard/boltcard/lnurlw"
	"github.com/boltcard/boltcard/ratelimit"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"
	"time"
)

var router = mux.NewRouter()

// the internal API listens here unless INTERNAL_API_LISTEN says otherwise
const default_internal_api_listen = "127.0.0.1:9001"

// requests per minute per caller allowed on the internal API
const internal_api_rate_limit = 60
const internal_api_rate_burst = 20

// internal_api_listen_address returns the address for the internal API
// listener. It defaults to the loopback interface, so that the internal API is
// not exposed to the network by a host without a firewall.
func internal_api_listen_address() string {
	listen := strings.TrimSpace(db.Get_setting("INTERNAL_API_LISTEN"))
	if listen == "" {
		return default_internal_api_listen
	}

	return listen
}

func main() {
	log_level := db.Get_setting("LOG_LEVEL")

	switch log_level {
	case "DEBUG":
		log.SetLevel(log.DebugLevel)
		log.Info("bolt card service started - debug log level")
	case "PRODUCTION":
		log.Info("bolt card service started - production log level")
	default:
		// log.Fatal calls os.Exit(1) after logging the error
		log.Fatal("error getting a valid LOG_LEVEL setting from the database")
	}

	log.SetFormatter(&log.JSONFormatter{
		DisableHTMLEscape: true,
	})

	var external_router = mux.NewRouter()
	var internal_router = mux.NewRouter()

	// external API

	// ping
	external_router.Path("/ping").Methods("GET").HandlerFunc(external_ping)
	// createboltcard
	external_router.Path("/new").Methods("GET").HandlerFunc(new_card_request)
	// lnurlw for pos
	external_router.Path("/ln").Methods("GET").HandlerFunc(lnurlw.Response)
	external_router.Path("/cb").Methods("GET").HandlerFunc(lnurlw.Callback)
	// lnurlp for lightning address
	external_router.Path("/.well-known/lnurlp/{name}").Methods("GET").HandlerFunc(lnurlp.Response)
	external_router.Path("/lnurlp/{name}").Methods("GET").HandlerFunc(lnurlp.Callback)

	// internal API
	// this creates cards and reads or wipes card settings, so it is not to be
	// exposed publicly
	// every function requires the INTERNAL_API_KEY to be presented as
	// `Authorization: Bearer <key>` and is rate limited per caller
	// it listens on the loopback interface unless INTERNAL_API_LISTEN says
	// otherwise, e.g. for use on a private virtual network between containers

	internal_limiter := ratelimit.New(internal_api_rate_limit, internal_api_rate_burst)

	protected := func(h http.HandlerFunc) http.HandlerFunc {
		return internal_limiter.Middleware(auth.Require_internal_api_key(h))
	}

	internal_router.Path("/ping").Methods("GET").HandlerFunc(internal_limiter.Middleware(internalapi.Internal_ping))
	internal_router.Path("/createboltcard").Methods("GET").HandlerFunc(protected(internalapi.Createboltcard))
	internal_router.Path("/createboltcardwithpin").Methods("GET").HandlerFunc(protected(internalapi.Createboltcardwithpin))
	internal_router.Path("/updateboltcard").Methods("GET").HandlerFunc(protected(internalapi.Updateboltcard))
	internal_router.Path("/updateboltcardwithpin").Methods("GET").HandlerFunc(protected(internalapi.Updateboltcardwithpin))
	internal_router.Path("/wipeboltcard").Methods("GET").HandlerFunc(protected(internalapi.Wipeboltcard))
	internal_router.Path("/getboltcard").Methods("GET").HandlerFunc(protected(internalapi.Getboltcard))

	port := db.Get_setting("HOST_PORT")
	if port == "" {
		port = "9000"
	}

	external_server := &http.Server{
		Handler:      external_router,
		Addr:         ":" + port, // consider adding host
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
	}

	internal_listen := internal_api_listen_address()

	internal_server := &http.Server{
		Handler:      internal_router,
		Addr:         internal_listen,
		WriteTimeout: 5 * time.Second,
		ReadTimeout:  5 * time.Second,
	}

	go func() {
		log.Info("external API listening on ", external_server.Addr)
		err := external_server.ListenAndServe()
		// log.Fatal calls os.Exit(1) after logging the error
		log.Fatal("external API server stopped: ", err)
	}()

	// the internal API port is only opened when the internal API is enabled
	// and an API key is configured, so a missing key cannot leave the card
	// creation functions reachable without authentication

	switch {
	case db.Get_setting("FUNCTION_INTERNAL_API") != "ENABLE":
		log.Info("internal API is disabled - no internal API port is opened")
	case !auth.Key_configured():
		log.Error("internal API is enabled but no INTERNAL_API_KEY of at least ",
			auth.Min_key_length, " characters is set - no internal API port is opened")
	default:
		if !strings.HasPrefix(internal_listen, "127.0.0.1:") &&
			!strings.HasPrefix(internal_listen, "localhost:") &&
			!strings.HasPrefix(internal_listen, "[::1]:") {
			log.Warn("the internal API is not listening on the loopback interface ",
				"- ensure access to ", internal_listen, " is restricted")
		}

		go func() {
			log.Info("internal API listening on ", internal_server.Addr)
			err := internal_server.ListenAndServe()
			log.Fatal("internal API server stopped: ", err)
		}()
	}

	select {}
}
