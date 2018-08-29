package main

import (
	"encoding/json"
	"errors"
	"math/rand"
	"net"
	"reflect"
	"strconv"
	"strings"
)

// NewGameSetting : new player registered new game setting
type NewGameSetting struct {
	GameType  string
	ActorType string
}

// ConnToNewGameSettingData : use for registering
type ConnToNewGameSettingData struct {
	Player  net.Conn
	Setting NewGameSetting
}

// ConnToRoomNameData : use for queue waiting for joining room player
type ConnToRoomNameData struct {
	Player   net.Conn
	RoomName string
}

// CreatedRoomNameData : use for receive Master Client
type CreatedRoomNameData struct {
	CreatedRoomName string
}

var connToNewGameSetting = make(map[net.Conn]NewGameSetting)
var connToRoomName = make(map[net.Conn]string)
var receivePlayerNewGameSetting = make(chan ConnToNewGameSettingData, 1024)
var registerPlayerNewGameSetting = make(chan map[net.Conn]NewGameSetting, 1024)
var registerPairedNotMasterClient = make(chan ConnToRoomNameData, 1024)
var callNotClientPlayerJoinRoom = make(chan string, 1024)
var usedRoomName = make([]string, 0)

func main() {

	go PairPlayer()
	go RegisterNewPlayer()
	go QueuePairedClient()
	go CallClinetJoinRoom()

	listener, err := net.Listen("tcp", ":80")

	if err != nil {
		println("err" + err.Error())
		return
	}

	println("Server Started:80")

	for {
		conn, err := listener.Accept()

		if err != nil {
			println("err" + err.Error())
			continue
		} else {
			println("Client Connected:" + conn.RemoteAddr().String())
			conn.Write([]byte("1"))
		}
		go ReceivePlayerMessage(conn)
	}
}

// ReceivePlayerMessage : analysis client's message
func ReceivePlayerMessage(conn net.Conn) {
	for {
		var buffer = make([]byte, 1024)

		n, err := conn.Read(buffer)

		if err != nil {

			if err.Error() != "EOF" {
				println(conn.RemoteAddr().String() + " connection error: " + err.Error())
			} else {
				delete(connToNewGameSetting, conn)
				delete(connToRoomName, conn)
				println(conn.RemoteAddr().String() + " disconneced")
			}

			return
		}

		var result = string(buffer[:n])
		println("--------------------")
		println("Receive:" + result)

		if strings.Contains(result, "CreatedRoomName") {
			var createdRoomData CreatedRoomNameData

			if err := json.Unmarshal([]byte(result), &createdRoomData); err == nil {
				println("-------------Set Room Created-------------")
				println("CreatedRoomName:" + createdRoomData.CreatedRoomName)
				callNotClientPlayerJoinRoom <- createdRoomData.CreatedRoomName
			}
		} else {
			var setting NewGameSetting

			if err := json.Unmarshal([]byte(result), &setting); err == nil {
				println("-------------Set Player-------------")
				println("Player:" + conn.RemoteAddr().String())
				println("GameType:" + setting.GameType)
				println("ActorType:" + setting.ActorType)

				var playerToSetting = ConnToNewGameSettingData{conn, setting}

				receivePlayerNewGameSetting <- playerToSetting
			}
		}

	}
}

// RegisterNewPlayer : when received new gamesetting, register it into map
func RegisterNewPlayer() {
	for {
		var receivedData = <-receivePlayerNewGameSetting
		connToNewGameSetting[receivedData.Player] = receivedData.Setting
		registerPlayerNewGameSetting <- connToNewGameSetting
	}
}

// PairPlayer : try pair player to a room when ever new player registered
func PairPlayer() {
	for {
		var currentConnToNewGameSetting = <-registerPlayerNewGameSetting
		var pve1v1TyperPlayers = make([]net.Conn, 0)
		var pve2v2TyperPlayers = make([]net.Conn, 0)
		var pvp1v1TyperPlayers = make([]net.Conn, 0)
		var pvp2v2TyperPlayers = make([]net.Conn, 0)
		println("----------PairPlayer----------")
		for player, setting := range currentConnToNewGameSetting {
			println("Set " + player.RemoteAddr().String())
			println("setting.GameType=" + setting.GameType)
			println("setting.ActorType=" + setting.ActorType)
			switch setting.GameType {
			case "pve_1v1":
				pve1v1TyperPlayers = append(pve1v1TyperPlayers, player)
			case "pve_2v2":
				pve2v2TyperPlayers = append(pve2v2TyperPlayers, player)
			case "pvp_1v1":
				pvp1v1TyperPlayers = append(pvp1v1TyperPlayers, player)
			case "pvp_2v2":
				pvp2v2TyperPlayers = append(pvp2v2TyperPlayers, player)
			}
		}

		for _, player := range pve1v1TyperPlayers {
			player.Write([]byte("{\"RoomName\":\"" + getRandomRoomName(currentConnToNewGameSetting[player].GameType) + "\", \"IsMasterClient\":true}"))
			delete(currentConnToNewGameSetting, player)
		}

		var pve2v2roomList = make([][]net.Conn, 0)
		var pvp1v1roomList = make([][]net.Conn, 0)
		var pvp2v2roomList = make([][]net.Conn, 0)

		var allPairedPlayers = make([]net.Conn, 0)

		var pve2v2PairedPlayers = checkPlayers(&pve2v2roomList, pve2v2TyperPlayers, currentConnToNewGameSetting, 2)
		var pvp1v1PairedPlayers = checkPlayers(&pvp1v1roomList, pvp1v1TyperPlayers, currentConnToNewGameSetting, 1)
		var pvp2v2PairedPlayers = checkPlayers(&pvp2v2roomList, pvp2v2TyperPlayers, currentConnToNewGameSetting, 2)

		for _, obj := range pve2v2PairedPlayers {
			allPairedPlayers = append(allPairedPlayers, obj)
		}

		for _, obj := range pvp1v1PairedPlayers {
			allPairedPlayers = append(allPairedPlayers, obj)
		}

		for _, obj := range pvp2v2PairedPlayers {
			allPairedPlayers = append(allPairedPlayers, obj)
		}

		for _, player := range allPairedPlayers {
			delete(currentConnToNewGameSetting, player)
		}
	}
}

func getRandomRoomName(gameType string) string {
	var roomID = rand.Intn(10000)
	var roomName = gameType + "_" + strconv.Itoa(roomID)
	var nameUsed, _ = Contain(usedRoomName, roomName)

	for nameUsed {
		roomID = rand.Intn(10000)
		nameUsed, _ = Contain(usedRoomName, roomName)
	}

	usedRoomName = append(usedRoomName, roomName)
	return roomName
}

// QueuePairedClient : when player was paired and they are not master-client, queue into list, waiting room created to join
func QueuePairedClient() {
	for {
		var pairedNotMasterClient = <-registerPairedNotMasterClient
		connToRoomName[pairedNotMasterClient.Player] = pairedNotMasterClient.RoomName
	}
}

// CallClinetJoinRoom : call not master clinet join room when room created
func CallClinetJoinRoom() {
	for {
		var roomName = <-callNotClientPlayerJoinRoom
		println("Calling client to join " + roomName)
		for player, roomNameInMap := range connToRoomName {
			println("Check " + player.RemoteAddr().String() + " is " + roomNameInMap)
			if roomNameInMap == roomName {
				println("Send " + player.RemoteAddr().String() + " Join " + roomNameInMap)
				player.Write([]byte("{\"JoinRoomName\":\"" + roomName + "\"}"))
				delete(connToRoomName, player)
			}
		}
	}
}

func checkPlayers(roomList *[][]net.Conn, players []net.Conn, connToSetting map[net.Conn]NewGameSetting, checkNum int) []net.Conn {
	var pairedRoomList = make([][]net.Conn, 0)
	for _, player := range players {
		println("checking " + player.RemoteAddr().String())
		var pairedRoom = pairIntoRoom(roomList, connToSetting, player, checkNum)
		if pairedRoom != nil {
			pairedRoomList = append(pairedRoomList, pairedRoom)
		}
	}

	var pairedPlayer = make([]net.Conn, 0)
	// do sth to paired room's all player
	for _, room := range pairedRoomList {
		var roomName = getRandomRoomName(connToSetting[room[0]].GameType)
		for playerIndex, player := range room {
			if playerIndex == 0 {
				player.Write([]byte("{\"RoomName\":\"" + roomName + "\", \"IsMasterClient\":true}"))
			} else {
				var notMasterClient ConnToRoomNameData
				notMasterClient.Player = player
				notMasterClient.RoomName = roomName
				registerPairedNotMasterClient <- notMasterClient
			}
			pairedPlayer = append(pairedPlayer, player)
		}
	}

	return pairedPlayer
}

func pairIntoRoom(roomList *[][]net.Conn, connToSetting map[net.Conn]NewGameSetting, player net.Conn, checkNum int) []net.Conn {
	for roomIndex, room := range *roomList {

		if len(room) == checkNum*2 {
			println("room#" + strconv.Itoa(roomIndex) + "has " + strconv.Itoa(len(room)) + " players, is full. Continued")
			continue
		}

		if isCanJoinRoom(room, connToSetting, player, checkNum) {
			(*roomList)[roomIndex] = append((*roomList)[roomIndex], player)
			println(player.RemoteAddr().String() + " joined room#" + strconv.Itoa(roomIndex) + ", room player number=" + strconv.Itoa(len((*roomList)[roomIndex])))
			if (len((*roomList)[roomIndex]) == checkNum*2 && strings.Contains(connToSetting[player].GameType, "pvp")) || (len((*roomList)[roomIndex]) == checkNum && strings.Contains(connToSetting[player].GameType, "pve")) {
				return (*roomList)[roomIndex]
			}
			return nil
		}
	}
	println(player.RemoteAddr().String() + " created room")
	var newRoom = make([]net.Conn, 0)
	newRoom = append(newRoom, player)
	*roomList = append(*roomList, newRoom)
	return nil
}

func isCanJoinRoom(room []net.Conn, connToSetting map[net.Conn]NewGameSetting, player net.Conn, checkNum int) bool {
	println(player.RemoteAddr().String() + " checking is can join room")
	var count = 0
	for _, playerInRoom := range room {
		if playerInRoom.RemoteAddr() == player.RemoteAddr() {
			continue
		} else {
			if strings.Contains(connToSetting[player].GameType, "pve") {
				if connToSetting[player].ActorType != connToSetting[playerInRoom].ActorType {
					println("this room is pve room and room actor type is " + connToSetting[playerInRoom].ActorType + ", player actor type is " + connToSetting[player].ActorType + ", can't join room")
					return false
				}
			}

			if connToSetting[player].ActorType == connToSetting[playerInRoom].ActorType {
				count++
			}

			if count >= checkNum {
				println("this room has " + strconv.Itoa(count) + " " + connToSetting[player].ActorType + ", can't join room")
				return false
			}
		}
	}
	return true
}

// Contain : checking object is in a slice/map/arrary
func Contain(obj interface{}, target interface{}) (bool, error) {
	targetValue := reflect.ValueOf(target)
	switch reflect.TypeOf(target).Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < targetValue.Len(); i++ {
			if targetValue.Index(i).Interface() == obj {
				return true, nil
			}
		}
	case reflect.Map:
		if targetValue.MapIndex(reflect.ValueOf(obj)).IsValid() {
			return true, nil
		}
	}

	return false, errors.New("not in array")
}
