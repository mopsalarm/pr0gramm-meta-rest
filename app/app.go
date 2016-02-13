package app

import (
  "encoding/json"
  "database/sql"
  "net/http"
  "strconv"
  "github.com/rcrowley/go-metrics"
  "github.com/gorilla/mux"
)

type HandleFunc func(*sql.DB, map[string]string, *http.Request) interface{}

type Handler struct {
  Database   *sql.DB
  HandleFunc HandleFunc
}

type Error struct {
  Status  int
  Message string
}

type TimeHandler struct {
  Timer   metrics.Timer
  Handler http.Handler
}

func (th TimeHandler) ServeHTTP(writer http.ResponseWriter, r *http.Request) {
  th.Timer.Time(func() {
    th.Handler.ServeHTTP(writer, r)
  })
}

func (appHandler Handler) ServeHTTP(writer http.ResponseWriter, r *http.Request) {
  vars := mux.Vars(r)
  result := appHandler.HandleFunc(appHandler.Database, vars, r)

  switch val := result.(type) {
  case Error:
    writer.WriteHeader(val.Status)
    writer.Write([]byte(val.Message))

  default:
    result, err := json.Marshal(result)
    if err != nil {
      panic(err)
    }

    writer.Header().Add("Content-Type", "application/json")
    writer.Header().Add("Content-Length", strconv.Itoa(len(result)))
    writer.Write(result)
  }
}
