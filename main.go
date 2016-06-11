package main // import "github.com/mopsalarm/pr0gramm-meta-rest"

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bobziuchkovski/writ"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"net/http"

	"database/sql"
	_ "github.com/lib/pq"

	"github.com/rcrowley/go-metrics"
	"github.com/vistarmedia/go-datadog"

	"github.com/mopsalarm/pr0gramm-meta-rest/app"
)

const SAMPLE_PERIOD = time.Minute

type Args struct {
	HelpFlag bool   `flag:"help" description:"Display this help message and exit"`
	Port     int    `option:"p, port" default:"8080" description:"The port to open the rest service on"`
	Postgres string `option:"postgres" default:"host=localhost user=postgres password=password sslmode=disable" description:"Postgres DSN for database connection"`
	Datadog  string `option:"datadog" description:"Datadog api key for reporting"`
}

type Route struct {
	name    string
	url     string
	handler app.HandleFunc
}

func main() {
	var err error
	args := &Args{}
	cmd := writ.New("webapp", args)

	// Use cmd.Decode(os.Args[1:]) in a real application
	_, _, err = cmd.Decode(os.Args[1:])
	if err != nil || args.HelpFlag {
		cmd.ExitHelp(err)
	}

	// open database connection
	db, err := sql.Open("postgres", args.Postgres)
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5*time.Minute)

	// check if it is valid
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}

	// get info about the runtime every few seconds
	metrics.RegisterRuntimeMemStats(metrics.DefaultRegistry)
	go metrics.CaptureRuntimeMemStats(metrics.DefaultRegistry, SAMPLE_PERIOD)

	if len(args.Datadog) > 0 {
		host, _ := os.Hostname()

		fmt.Printf("Starting datadog reporter on host %s\n", host)
		go datadog.New(host, args.Datadog).DefaultReporter().Start(SAMPLE_PERIOD)
	}

	router := mux.NewRouter().StrictSlash(true)

	routes := []Route{
		Route{"user", "/user/{user}", handleUser},
		Route{"user-suggest", "/user/suggest/{prefix}", handleUserSuggest},
	}

	for _, route := range routes {
		timer := metrics.NewRegisteredTimer("pr0gramm.meta.webapp.request."+route.name, nil)
		router.Handle(route.url, app.TimeHandler{timer, app.Handler{db, route.handler}})
	}

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", args.Port),
		handlers.RecoveryHandler()(
			handlers.LoggingHandler(os.Stdout,
				handlers.CORS()(router)))))
}
