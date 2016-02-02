package helen

import (
	"github.com/TF2Stadium/Helen/models"
	helen "github.com/TF2Stadium/Helen/rpc"
	"github.com/TF2Stadium/Pauling/helpers"
)

// GetPlayerID returns a player ID (primary key), given their Steam Community id
func GetPlayerID(steamid string) (id uint) {
	Call("Helen.GetPlayerID", steamid, &id)
	return
}

// GetTeam returns the player's team, given the player's steamid and the lobby id
func GetTeam(lobbyID uint, lobbyType models.LobbyType, steamID string) (team string) {
	Call("Helen.GetTeam", helen.Args{LobbyID: lobbyID, Type: lobbyType, SteamID: steamID}, &team)
	return
}

// GetSlotSteamID returns the steam ID for the player occupying the given slot
func GetSlotSteamID(team, class string, lobbyID uint, lobbyType models.LobbyType) (steamID string) {
	args := helen.Args{
		LobbyID: lobbyID,
		Type:    lobbyType,

		Team:  team,
		Class: class,
	}

	Call("Helen.GetSteamIDFromSlot", args, &steamID)

	return
}

// GetName returns the name for a plyer given their steam ID
func GetName(steamID string) (name string) {
	Call("Helen.GetNameFromSteamID", steamID, &name)
	return
}

func IsAllowed(lobbyID uint, steamID string) (allowed bool) {
	err := Call("Helen.IsAllowed", helen.Args{LobbyID: lobbyID, SteamID: steamID}, &allowed)
	if err != nil {
		helpers.Logger.Error(err.Error())
	}
	return
}

func IsReported(lobbyID uint, steamID string) (repped bool) {
	err := Call("Helen.IsReported", helen.Args{LobbyID: lobbyID, SteamID: steamID}, &repped)
	if err != nil {
		helpers.Logger.Error(err.Error())
	}
	return
}

func SetSecret(lobbyID uint, secret string) {
	err := Call("Helen.SetSecret", helen.Args{LobbyID: lobbyID, LogSecret: secret}, &struct{}{})
	if err != nil {
		helpers.Logger.Error(err.Error())
	}

	return
}
