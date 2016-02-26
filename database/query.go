package database

import (
	"github.com/TF2Stadium/Helen/models"
	"github.com/TF2Stadium/Pauling/helpers"
)

func GetPlayerID(commID string) (playerID uint) {
	err := db.QueryRow("SELECT name FROM player_slots WHERE steam_id = $1 LIMIT 1", commID).Scan(&playerID)
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
	var state models.LobbyState
	var needsSub bool

	err := db.QueryRow("SELECT state, needs_sub FROM player_slots WHERE lobby_id = $1 AND steam_id = $2", lobbyID, commID).Scan(&state, &needsSub)
	if err != nil {
		return false, "You're not in the lobby."
	}

	if !needsSub && state == models.LobbyStateWaiting {
		return false, "The lobby hasn't started yet."
	}

	return true, ""
}

func IsReported(lobbyID uint, commID string) (allowed bool) {
	err := db.QueryRow("SELECT needs_sub FROM player_slots WHERE lobby_id = $1 AND steam_id = $2", lobbyID, commID).Scan(&allowed)
	if err != nil {
		helpers.Logger.Error(err.Error())
	}
	return
}

func GetTeam(lobbyID uint, lobbyType models.LobbyType, commID string) (team string) {
	var slot int
	err := db.QueryRow("SELECT slot FROM player_slots WHERE lobby_id = $1 AND steam_id = $2", lobbyID, commID).Scan(&slot)
	if err != nil {
		helpers.Logger.Error(err.Error())
	}
	team, _, _ = models.LobbyGetSlotInfoString(lobbyType, slot)
	return
}

func GetSteamIDFromSlot(team, class string, lobbyID uint, lobbyType models.LobbyType) (commID string) {
	slot, _ := models.LobbyGetPlayerSlot(lobbyType, team, class)
	err := db.QueryRow("SELECT steam_id FROM player_slots WHERE lobby_id = $1 AND slot = $2", lobbyID, slot).Scan(&commID)
	if err != nil {
		helpers.Logger.Error(err.Error())
	}
	return
}

func GetNameFromSteamID(commID string) (name string) {
	err := db.QueryRow("SELECT name FROM player_slots WHERE steam_id = $1", commID).Scan(&name)
	if err != nil {
		helpers.Logger.Error(err.Error())
	}
	return
}
