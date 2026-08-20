package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/boltcard/boltcard/auth"
	"github.com/boltcard/boltcard/db"
	"github.com/boltcard/boltcard/internalapi"
	"github.com/boltcard/boltcard/lnurlp"
	"github.com/boltcard/boltcard/lnurlw"
	"github.com/boltcard/boltcard/ratelimit"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

var router = mux.NewRouter()

// the internal API listens here unless INTERNAL_API_LISTEN says otherwise
const default_internal_api_listen = "127.0.0.1:9001"

// requests per minute per caller allowed on the internal API
const internal_api_rate_limit = 60
const internal_api_rate_burst = 20

// defaults for the external API, used when the setting is not set
//
// the per caller limits are well above what a point of sale or a card
// programming app needs, so they turn away abuse rather than shaping traffic
const default_external_rate_limit = 120
const default_external_rate_burst = 40

// the number of external requests handled at once is capped, because each one
// opens database connections of its own
const default_external_max_concurrent = 32

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

// setting_int reads a whole number setting, falling back to a default value
// when it is not set or cannot be read.
func setting_int(setting_name string, default_value int) int {
	value_str := strings.TrimSpace(db.Get_setting(setting_name))
	if value_str == "" {
		return default_value
	}

	value, err := strconv.Atoi(value_str)
	if err != nil {
		log.Warn("the ", setting_name, " setting is not a valid integer - using ", default_value)
		return default_value
	}

	return value
}

func main() {
	// settings are read on every request, so they are held briefly rather than
	// read from the database each time
	// SETTING_CACHE_SEC of 0 turns that off, at the cost of database load
	setting_cache_sec := db.Default_setting_cache_sec
	if setting_cache_sec_str := strings.TrimSpace(db.Get_setting_now("SETTING_CACHE_SEC")); setting_cache_sec_str != "" {
		if seconds, err := strconv.Atoi(setting_cache_sec_str); err == nil {
			setting_cache_sec = seconds
		} else {
			log.Warn("the SETTING_CACHE_SEC setting is not a valid integer - using ",
				db.Default_setting_cache_sec)
		}
	}
	db.Set_setting_cache_seconds(setting_cache_sec)

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

	if setting_cache_sec > 0 {
		log.Info("settings are read from the database at most every ", setting_cache_sec, "s")
	} else {
		log.Info("settings are read from the database on every use")
	}

	var external_router = mux.NewRouter()
	var internal_router = mux.NewRouter()

	// external API
	//
	// every function that reads the database is rate limited per caller and
	// runs under a cap on how many requests are handled at once, so that a
	// flood cannot use up the database connections and stop the service
	//
	// TRUSTED_PROXY_COUNT tells the service how many reverse proxies are in
	// front of it, so that callers are told apart by their own address rather
	// than all counted as the proxy

	trusted_proxy_count := setting_int("TRUSTED_PROXY_COUNT", 0)

	external_limiter := ratelimit.New(
		setting_int("EXTERNAL_RATE_LIMIT_PER_MIN", default_external_rate_limit),
		setting_int("EXTERNAL_RATE_BURST", default_external_rate_burst))
	external_limiter.Key_func = ratelimit.Key_func_for_proxies(trusted_proxy_count)

	external_concurrency := ratelimit.New_concurrency_limiter(
		setting_int("EXTERNAL_MAX_CONCURRENT", default_external_max_concurrent))

	limited := func(h http.HandlerFunc) http.HandlerFunc {
		return external_concurrency.Middleware(external_limiter.Middleware(h))
	}

	if trusted_proxy_count == 0 {
		log.Info("TRUSTED_PROXY_COUNT is not set - external callers are told ",
			"apart by the address the service sees, which is the reverse proxy ",
			"where one is used")
	}

	// ping
	// not limited, so that health checks answer while the service is busy
	external_router.Path("/ping").Methods("GET").HandlerFunc(external_ping)
	// createboltcard
	external_router.Path("/new").Methods("GET").HandlerFunc(limited(new_card_request))
	// lnurlw for pos
	external_router.Path("/ln").Methods("GET").HandlerFunc(limited(lnurlw.Response))
	external_router.Path("/cb").Methods("GET").HandlerFunc(limited(lnurlw.Callback))
	// lnurlp for lightning address
	external_router.Path("/.well-known/lnurlp/{name}").Methods("GET").HandlerFunc(limited(lnurlp.Response))
	external_router.Path("/lnurlp/{name}").Methods("GET").HandlerFunc(limited(lnurlp.Callback))
	// payment approval from the card holder's notification
	external_router.Path("/approve").Methods("GET").HandlerFunc(limited(approve_request))

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
	internal_router.Path("/updateboltcardpayment").Methods("GET").HandlerFunc(protected(internalapi.Updateboltcardpayment))

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
