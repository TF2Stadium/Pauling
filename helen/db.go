package helen

import (
	"github.com/TF2Stadium/Helen/models"
	helen "github.com/TF2Stadium/Helen/rpc"
)

// GetPlayerID returns a player ID (primary key), given their Steam Community id
func GetPlayerID(steamid string) (id uint) {
	Client.Call("Helen.GetPlayerID", steamid, &id)
	return
}

// GetTeam returns the player's team, given the player's steamid and the lobby id
func GetTeam(lobbyID uint, lobbyType models.LobbyType, steamID string) (team string) {
	Client.Call("Helen.GetTeam", helen.Args{LobbyID: lobbyID, Type: lobbyType, SteamID: steamID}, &team)
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

	Client.Call("Helen.GetSteamIDFromSlot", args, &steamID)

	return
}

// GetName returns the name for a plyer given their steam ID
func GetName(steamID string) (name string) {
	Client.Call("Helen.GetNameFromSteamID", steamID, &name)
	return
}

func IsAllowed(lobbyID uint, steamID string) (allowed bool) {
	Client.Call("Helen.IsAllowed", helen.Args{LobbyID: lobbyID, SteamID: steamID}, &allowed)
	return
}
