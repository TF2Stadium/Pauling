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
	var playerID uint
	var needsSub bool
	db.QueryRow("SELECT id FROM players WHERE steam_id = $1", commID).Scan(&playerID)
	err := db.QueryRow("SELECT state, needs_sub FROM lobby_slots WHERE lobby_id = $1 AND player_id = $2", lobbyID, playerID).Scan(&state, &needsSub)
	if err != nil || needsSub {
		return false, "You're not in the lobby."
	}

	if !needsSub && state == models.LobbyStateWaiting {
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

func GetTeam(lobbyID uint, lobbyType models.LobbyType, commID string) (team string) {
	var slot int
	err := db.QueryRow("SELECT lobby_slots.slot FROM lobby_slots INNER JOIN players ON lobby_slots.player_id = players.id WHERE lobby_slots.lobby_id = $1 AND players.steam_id = $2", lobbyID, commID).Scan(&slot)
	if err != nil {
		helpers.Logger.Error(err.Error())
	}
	team, _, _ = models.LobbyGetSlotInfoString(lobbyType, slot)
	return
}

func GetSteamIDFromSlot(team, class string, lobbyID uint, lobbyType models.LobbyType) (commID string) {
	slot, _ := models.LobbyGetPlayerSlot(lobbyType, team, class)
	err := db.QueryRow("SELECT players.steam_id FROM players INNER JOIN lobby_slots ON players.id = lobby_slots.player_id WHERE lobby_slots.lobby_id = $1 AND lobby_slots.slot = $2", lobbyID, slot).Scan(&commID)
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
