package app

import (
  "encoding/json"
  "database/sql"
  "net/http"
  "strconv"
)

type HandleFunc func(*sql.DB, *http.Request) interface{}

type Handler struct {
  Database   *sql.DB
  HandleFunc HandleFunc
}

type Error struct {
  Status  int
  Message string
}

func (appHandler Handler) ServeHTTP(writer http.ResponseWriter, r *http.Request) {
  result := appHandler.HandleFunc(appHandler.Database, r)
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