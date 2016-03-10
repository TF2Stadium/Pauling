package server

import (
	"fmt"

	"github.com/TF2Stadium/Pauling/helpers"
	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"
)

var (
	db *gorm.DB
)

type report struct {
	ID      uint
	LobbyID uint
	Source  string
	Target  string
}

type repError struct {
	source string
	target string
}

func (e repError) Error() string {
	return fmt.Sprintf("%s has already repped %s", e.source, e.target)
}

//CreateDB initializes the sqlite database
func CreateDB() {
	var err error
	db, err = gorm.Open("sqlite3", "./pauling.db")
	if err != nil {
		helpers.Logger.Fatal(err)
	}

	// id | source_player_id | target_player_id | lobby_id

	err = db.AutoMigrate(&report{}).Error
	if err != nil {
		helpers.Logger.Fatal(err)
	}

}

func hasReported(source, target string, lobbyID uint) bool {
	var count int
	db.Table("reports").Where(&report{LobbyID: lobbyID, Source: source, Target: target}).Count(&count)
	return count != 0
}

func newReport(source, target string, lobbyID uint) error {
	if hasReported(source, target, lobbyID) {
		return &repError{source, target}
	}

	rep := &report{LobbyID: lobbyID, Source: source, Target: target}
	return db.Table("reports").Create(rep).Error
}

//ResetReportCount resets the !rep count for the given player in lobby lobbyID
func ResetReportCount(target string, lobbyID uint) error {
	err := db.Table("reports").Where(&report{Target: target, LobbyID: lobbyID}).Delete(&report{}).Error
	return err
}

func countReports(target string, lobbyID uint) int {
	var count int

	db.Table("reports").Where(&report{LobbyID: lobbyID, Target: target}).Count(&count)
	return count
}
