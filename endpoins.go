package main

import (
	"database/sql"
	_ "github.com/lib/pq"
	"net/http"
	"strings"
	"time"

	"github.com/mopsalarm/pr0gramm-meta-rest/app"
)

func queryOrPanic(db *sql.DB, query string, args ...interface{}) *sql.Rows {
	rows, err := db.Query(query, args...)
	if err != nil {
		panic(err)
	}

	return rows
}

func handleUser(db *sql.DB, vars map[string]string, req *http.Request) interface{} {
	minTimestamp := time.Now().Add(-7 * 24 * time.Hour).Unix()

	rows := queryOrPanic(db, `SELECT user_score.timestamp, user_score.score
    FROM user_score, users
    WHERE lower(users.name)=lower($1) AND users.id=user_score.user_id AND user_score.timestamp>$2`,
		vars["user"], minTimestamp)

	defer rows.Close()

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

func handleUserSuggest(db *sql.DB, vars map[string]string, req *http.Request) interface{} {
	prefix := strings.Replace(vars["prefix"], "%", "", 0) + "%"
	if len(prefix) < 3 {
		return app.Error{http.StatusPreconditionFailed, "Need at least 3 characters"}
	}

	rows := queryOrPanic(db,
		"SELECT name FROM users WHERE lower(name) LIKE lower($1) ORDER BY score DESC LIMIT 20",
		prefix)

	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			panic(err)
		}

		names = append(names, name)
	}

	return UserSuggestResponse{names}
}
