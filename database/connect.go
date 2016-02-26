package database

import (
	"database/sql"
	"net/url"

	"github.com/TF2Stadium/Pauling/config"
	"github.com/TF2Stadium/Pauling/helpers"
	_ "github.com/lib/pq"
)

var db *sql.DB

func Connect() {
	DBUrl := url.URL{
		Scheme:   "postgres",
		Host:     config.Constants.DBAddr,
		Path:     config.Constants.DBDatabase,
		RawQuery: "sslmode=disable",
	}

	helpers.Logger.Debug("Connecting to DB on %s", DBUrl.String())

	DBUrl.User = url.UserPassword(config.Constants.DBUsername, config.Constants.DBPassword)
	var err error

	db, err = sql.Open("postgres", DBUrl.String())
	if err != nil {
		helpers.Logger.Fatal(err)
	}

	helpers.Logger.Debug("Connected.")
}
