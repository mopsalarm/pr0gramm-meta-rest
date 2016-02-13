package main // import "github.com/mopsalarm/pr0gramm-meta-rest"

import (
  "os"
  "fmt"
  "log"
  "time"
  "strings"
  "regexp"
  "sync"

  "github.com/bobziuchkovski/writ"

  "net/http"
  "github.com/gorilla/mux"
  "github.com/gorilla/handlers"

  "database/sql"
  _ "github.com/lib/pq"

  "github.com/rcrowley/go-metrics"
  "github.com/vistarmedia/go-datadog"

  "github.com/mopsalarm/pr0gramm-meta-rest/app"
)

const SAMPLE_PERIOD = time.Minute

type Args struct {
  HelpFlag  bool   `flag:"help" description:"Display this help message and exit"`
  Verbosity int    `flag:"v, verbose" description:"Display verbose output"`
  Port      int    `option:"p, port" default:"8080" description:"The port to open the rest service on"`
  Postgres  string `option:"postgres" default:"host=localhost user=postgres password=password sslmode=disable" description:"Postgres DSN for database connection"`
  Datadog   string `option:"datadog" description:"Datadog api key for reporting"`
}

func queryOrPanic(db *sql.DB, query string, args ...interface{}) *sql.Rows {
  rows, err := db.Query(query, args...)
  if err != nil {
    panic(err)
  }

  return rows
}

func queryReposts(db *sql.DB, itemIds string) []int64 {
  rows := queryOrPanic(db, fmt.Sprintf(`SELECT item_id FROM tags
    WHERE item_id IN (%s) AND +confidence>0.3 AND lower(tag)='repost'
    LIMIT 150`, itemIds))

  result := make([]int64, 0, 100)
  for rows.Next() {
    var itemId int64
    if err := rows.Scan(&itemId); err != nil {
      panic(err)
    }

    result = append(result, itemId)
  }

  return result
}

func querySizes(db *sql.DB, itemIds string) []SizeInfo {
  rows := queryOrPanic(db, fmt.Sprintf("SELECT id, width, height FROM sizes WHERE id IN (%s) LIMIT 150", itemIds))

  sizeInfos := make([]SizeInfo, 0, 100)
  for rows.Next() {
    var info SizeInfo
    if err := rows.Scan(&info.Id, &info.Width, &info.Height); err != nil {
      panic(err)
    }

    sizeInfos = append(sizeInfos, info)
  }

  return sizeInfos
}

func queryPreviews(db *sql.DB, itemIds string) []PreviewInfo {
  rows := queryOrPanic(db,
    "SELECT id, width, height, encode(preview, 'base64') FROM item_previews WHERE id IN (%s) LIMIT 150",
    itemIds)

  infos := make([]PreviewInfo, 0, 100)
  for rows.Next() {
    var info PreviewInfo
    if err := rows.Scan(&info.Id, &info.Width, &info.Height, &info.Pixels); err != nil {
      panic(err)
    }

    infos = append(infos, info)
  }

  return infos
}

func handleUser(db *sql.DB, req *http.Request) interface{} {
  vars := mux.Vars(req)
  minTimestamp := time.Now().Add(-7 * 24 * time.Hour).Unix()

  rows := queryOrPanic(db, `SELECT user_score.timestamp, user_score.score
    FROM user_score, users
    WHERE lower(users.name)=lower($1) AND users.id=user_score.user_id AND user_score.timestamp>$2`,
    vars["user"], minTimestamp)

  result := UserResponse{}

  for rows.Next() {
    value := make([]int32, 2)
    if err := rows.Scan(&value[0], &value[1]); err != nil {
      panic(err)
    }

    result.BenisHistory = append(result.BenisHistory, value)
  }

  return result
}

func handleUserSuggest(db *sql.DB, req *http.Request) interface{} {
  vars := mux.Vars(req)

  prefix := strings.Replace(vars["prefix"], "%", "", 0) + "%"
  if len(prefix) < 3 {
    return app.Error{http.StatusPreconditionFailed, "Need at least 3 characters"}
  }

  rows := queryOrPanic(db,
    "SELECT name FROM users WHERE lower(name) LIKE lower($1) ORDER BY score DESC LIMIT 20",
    prefix)

  var names []string
  for rows.Next() {
    var name string
    if err := rows.Scan(&name); err != nil {
      panic(err);
    }

    names = append(names, name)
  }

  return UserSuggestResponse{names}
}

func handleItems(db *sql.DB, req *http.Request) interface{} {
  startTime := time.Now()

  // validate input
  itemIds := req.FormValue("ids")
  if match, err := regexp.Match(`^\d+(?:,\d+)*$`, []byte(itemIds)); !match || err != nil {
    return app.Error{400, "Invalid value for parameter ids"}
  }

  var response InfoResponse

  // we create a wait group and do all three queries in parallel
  var wg sync.WaitGroup
  wg.Add(3)

  go func() {
    defer wg.Done()
    response.Reposts = queryReposts(db, itemIds)
  }()

  go func() {
    defer wg.Done()
    response.Sizes = querySizes(db, itemIds)
  }()

  go func() {
    defer wg.Done()
    response.Previews = queryPreviews(db, itemIds)
  }()

  // at the end we wait for all of them to finish
  wg.Wait()

  response.Duration = time.Since(startTime).Seconds()
  return response
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
  db.SetMaxOpenConns(4)
  if err != nil {
    log.Fatal(err)
  }

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
    Route{"items", "/items", handleItems},
    Route{"user", "/user/{user}", handleUser},
    Route{"user-suggest", "/user/suggest/{prefix}", handleUserSuggest},
  }

  for _, route := range routes {
    timer := metrics.NewRegisteredTimer("pr0gramm.meta.webapp.request." + route.name, nil)
    router.Handle(route.url, app.TimeHandler{timer, app.Handler{db, route.handler}})
  }

  log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", args.Port),
    handlers.RecoveryHandler()(
      handlers.LoggingHandler(os.Stdout,
        handlers.CORS()(router)))))
}
