package server

import (
	"database/sql"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/TF2Stadium/Pauling/helpers"
	_ "github.com/mattn/go-sqlite3"
)

var (
	db     *sql.DB
	lastID uint32
)

type repError struct {
	source string
	target string
}

func (e repError) Error() string {
	return fmt.Sprintf("%s has already repped %s", e.source, e.target)
}

//CreateDB initializes the sqlite database
func CreateDB() {
	os.Remove("./pauling.db")

	var err error
	db, err = sql.Open("sqlite3", "./pauling.db")
	if err != nil {
		helpers.Logger.Fatal(err)
	}

	// id | source_player_id | target_player_id | lobby_id

	_, err = db.Exec("CREATE TABLE reports (id integer not null primary key, source_player_id varchar, target_player_id varchar, lobby_id integer not null)")
	if err != nil {
		helpers.Logger.Fatal(err)
	}
}

func hasReported(source, target string, lobbyID uint) bool {
	rows, err := db.Query("SELECT id FROM reports WHERE source_player_id = $1 AND target_player_id = $2 AND lobby_id = $3", source, target, lobbyID)
	if err != nil {
		helpers.Logger.Error(err.Error())
	}
	return rows.Next()
}

func newReport(source, target string, lobbyID uint) error {
	if hasReported(source, target, lobbyID) {
		return &repError{source, target}
	}

	_, err := db.Exec("INSERT INTO reports(id, source_player_id, target_player_id, lobby_id) values($1, $2, $3, $4)",
		atomic.AddUint32(&lastID, 1), source, target, lobbyID)

	return err
}

func resetReportCount(target string, lobbyID uint) error {
	_, err := db.Exec("DELETE FROM reports WHERE target_player_id = $1 AND lobby_id = $2", target, lobbyID)
	return err
}

func countReports(target string, lobbyID uint) int {
	rows, err := db.Query("SELECT id FROM reports WHERE target_player_id = $1 AND lobby_id = $2", target, lobbyID)
	if err != nil {
		helpers.Logger.Error(err.Error())
		return 0
	}

	var count int
	for rows.Next() {
		count++
	}

	return count
}
