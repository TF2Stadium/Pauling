package helen

import (
	"net/rpc"

	"github.com/TF2Stadium/Helen/models"
	helen "github.com/TF2Stadium/Helen/rpc"
	"github.com/TF2Stadium/Pauling/helpers"
)

func CheckConnection() {
	client, err := rpc.DialHTTP("tcp", "localhost:"+helpers.PortHelen)
	if err != nil {
		helpers.Logger.Fatal(err.Error())
	}
	err = client.Call("Helen.Test", struct{}{}, &struct{}{})
	if err != nil {
		helpers.Logger.Fatal(err.Error())
	}
	helpers.Logger.Debug("Able to connect to Helen")
}

func Call(method string, args interface{}, reply interface{}) error {
	client, err := rpc.DialHTTP("tcp", "localhost:"+helpers.PortHelen)
	if err != nil {
		helpers.Logger.Error(err.Error())
		return err
	}
	err = client.Call(method, args, reply)
	client.Close()

	return err
}

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
