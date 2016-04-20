package database

import (
	"github.com/TF2Stadium/Helen/models/lobby/format"
	"github.com/TF2Stadium/Pauling/helpers"
)

func GetPlayerID(commID string) (playerID uint) {
	err := db.QueryRow("SELECT name FROM players WHERE steam_id = $1", commID).Scan(&playerID)
	if err != nil {
		helpers.Logger.Error(err.Error())
	}
	return
}

func SetSecret(secret string, id uint) {
	_, err := db.Exec("UPDATE server_records SET log_secret = $1 WHERE id = $2", secret, id)
	if err != nil {
		helpers.Logger.Error(err.Error())
	}
}

func IsAllowed(lobbyID uint, commID string) (bool, string) {
	var state int
	var playerID uint
	var needsSub bool
	db.QueryRow("SELECT id FROM players WHERE steam_id = $1", commID).Scan(&playerID)
	err := db.QueryRow("SELECT needs_sub FROM lobby_slots WHERE lobby_id = $1 AND player_id = $2", lobbyID, playerID).Scan(&needsSub)
	if err != nil || needsSub {
		return false, "You're not in the lobby."
	}

	db.QueryRow("SELECT state FROM lobbies WHERE id = $1", lobbyID).Scan(&state)

	// 1 == Waiting
	if state == 1 {
		return false, "The lobby hasn't started yet."
	}

	return true, ""
}

func IsReported(lobbyID uint, commID string) (reported bool) {
	err := db.QueryRow("SELECT lobby_slots.needs_sub FROM lobby_slots INNER JOIN players ON lobby_slots.player_id = players.id WHERE lobby_slots.lobby_id = $1 AND players.steam_id = $2", lobbyID, commID).Scan(&reported)
	if err != nil {
		helpers.Logger.Error(err.Error())
	}
	return
}

func GetTeam(lobbyID uint, lobbyType format.Format, commID string) (team string) {
	var slot int
	err := db.QueryRow("SELECT lobby_slots.slot FROM lobby_slots INNER JOIN players ON lobby_slots.player_id = players.id WHERE lobby_slots.lobby_id = $1 AND players.steam_id = $2", lobbyID, commID).Scan(&slot)
	if err != nil {
		helpers.Logger.Error(err.Error())
	}
	team, _, _ = format.GetSlotTeamClass(lobbyType, slot)
	return
}

func GetSteamIDFromSlot(team, class string, lobbyID uint, lobbyType format.Format) (string, error) {
	slot, err := format.GetSlot(lobbyType, team, class)
	if err != nil {
		return "", err
	}

	var playerid uint
	err = db.QueryRow("SELECT player_id FROM lobby_slots WHERE lobby_id = $1 AND slot = $2", lobbyID, slot).Scan(&playerid)
	if err != nil {
		helpers.Logger.Errorf("#%d: Error while getting steamid for %s %s: %v", lobbyID, team, class, err)
	}

	var commID string
	err = db.QueryRow("SELECT steam_id FROM players WHERE id = $1", playerid).Scan(&commID)
	if err != nil {
		helpers.Logger.Errorf("#%d: Error while getting steamid for %s %s: %v", lobbyID, team, class, err)
	}

	return commID, nil
}

func GetNameFromSteamID(commID string) (name string) {
	err := db.QueryRow("SELECT name FROM players WHERE steam_id = $1", commID).Scan(&name)
	if err != nil {
		helpers.Logger.Error(err.Error())
	}
	return
}
