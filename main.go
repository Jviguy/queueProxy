package main

import (
	"container/list"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jviguy/gopher_query"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"queueProxy/utils"
	"strconv"
	"strings"
	"sync"
)

var servers = make(map[string]string)
var queues = make(map[string]*list.List)
var qc = gopher_query.NewClient()

func localLoop(conn *minecraft.Conn)  {
	ip := strings.Split(conn.ClientData().ServerAddress, ":")[0]
	port, _ := strconv.ParseInt(strings.Split(conn.ClientData().ServerAddress, ":")[1], 10, 16)
	err := conn.WritePacket(&packet.Transfer{Port: uint16(port), Address: ip})
	if err != nil {
		return
	}
	defer conn.Close()
}

func main() {
	listener, err := minecraft.ListenConfig{}.Listen("raknet", "0.0.0.0:19132")
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	for {
		c, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		server := servers[c.(*minecraft.Conn).IdentityData().XUID]
		canJoin := CanJoinServer(c.(*minecraft.Conn), server)
		if server != "" && canJoin {
			go handleConn(c.(*minecraft.Conn), listener)
		} else {
			go spawnLobby(c.(*minecraft.Conn), !canJoin)
		}
	}
}

func CanJoinServer(conn *minecraft.Conn, server string)  bool {
	if queue, ok := queues[server]; ok {
		if queue.Front().Value.(string) != conn.IdentityData().XUID {
			return false
		}
	}
	if server != "" {
		data, err := qc.ShortQuery(server)
		if err != nil {
			panic(err)
		}
		if data.PlayerCount == data.MaxPlayerCount {
			return false
		}
	}
	return true
}

func Find(li *list.List, x string) int {
	i := 0
	for e := li.Front(); e != nil; e=e.Next() {
		i++
		if x == e.Value.(string) {
			return i
		}
	}
	return li.Len()
}

func spawnLobby(conn *minecraft.Conn, inQueue bool) {
	var g sync.WaitGroup
	g.Add(1)
	go func() {
		if err := conn.StartGame(conn.GameData()); err != nil {
			panic(err)
		}
		g.Done()
	}()
	g.Wait()
	if inQueue {
		conn.WritePacket(&packet.Text{Message: "That server is currently full we have placed you into a queue for now."})
	}
	original_pos := 0
	if inQueue {
		if queue , ok := queues[servers[conn.IdentityData().XUID]]; ok {
			original_pos = Find(queue, conn.IdentityData().XUID)
		}
	}
	for {
		if inQueue {
			if queue, ok := queues[servers[conn.IdentityData().XUID]]; ok {
				new_pos := Find(queue, conn.IdentityData().XUID)
				if original_pos != new_pos {
					original_pos = new_pos
					conn.WritePacket(&packet.Text{Message: fmt.Sprintf("You are: %d in the queue", original_pos)})
				}
			}
			if CanJoinServer(conn, servers[conn.IdentityData().XUID]) {
				//First in queue and server is open
				conn.WritePacket(&packet.Text{Message: "Server is now not full and you are first in queue joining now!", TextType: packet.TextTypeRaw})
				localLoop(conn)
				return
			}
		}
		pk, err := conn.ReadPacket()
		if err != nil {
			return
		}
		fmt.Printf("%T\n", pk)
		switch pk := pk.(type) {
		//Form responses
		case *packet.ModalFormResponse:
			data := make([]interface{}, 2)
			err := json.Unmarshal(pk.ResponseData, &data)
			if err != nil {
				return
			}
			servers[conn.IdentityData().XUID] = data[0].(string) + ":" + data[1].(string)
			if CanJoinServer(conn, servers[conn.IdentityData().XUID]) {
				localLoop(conn)
				return
			} else {
				conn.WritePacket(&packet.Text{Message: "That server is currently full we have placed you into a queue for now."})
				if queue, ok := queues[servers[conn.IdentityData().XUID]]; ok {
					queue.PushBack(conn.IdentityData().XUID)
				} else {
					queues[servers[conn.IdentityData().XUID]] = list.New()
					queues[servers[conn.IdentityData().XUID]].PushBack(conn.IdentityData().XUID)
				}
				inQueue = true
			}
		//Commands
		case *packet.CommandRequest:
			if pk.CommandLine[0] == '/' {
				if pk.CommandLine[1:] == "proxyhelp" {
					//send help menu
				}
				if pk.CommandLine[1:] == "connect" && !inQueue {
					data := utils.CreateCustomForm("Connect to a server", utils.CustomFormField{Type: "input", Text: "IP",
						PlaceHolder: "", Default: ""}, utils.CustomFormField{Type: "input", Text: "Port", Default: "19132",
						PlaceHolder: ""})
					err := conn.WritePacket(&packet.ModalFormRequest{FormID: 1, FormData: data})
					if err != nil {
						panic(err)
					}
				}
				if pk.CommandLine[1:] == "ql" {
					inQueue = false
					queues[servers[conn.IdentityData().XUID]].Remove(&list.Element{Value: conn.IdentityData().XUID})
				}
			}
		}
	}
}

func handleConn(conn *minecraft.Conn, listener *minecraft.Listener) {
	serverConn, err := minecraft.Dialer{
		LoginPacket: conn.LoginPacket,
		ClientData:  conn.ClientData(),
	}.Dial("raknet", servers[conn.IdentityData().XUID])
	if err != nil {
		panic(err)
	}
	servers[conn.IdentityData().XUID] = ""
	var g sync.WaitGroup
	g.Add(2)
	go func() {
		if err := conn.StartGame(serverConn.GameData()); err != nil {
			panic(err)
		}
		g.Done()
	}()
	go func() {
		if err := serverConn.DoSpawn(); err != nil {
			panic(err)
		}
		g.Done()
	}()
	g.Wait()

	go func() {
		defer listener.Disconnect(conn, "connection lost")
		defer serverConn.Close()
		for {
			pk, err := conn.ReadPacket()
			if err != nil {
				return
			}
			switch pk := pk.(type) {
			case *packet.ModalFormResponse:
				if pk.FormID == 69696969 {
					data := make([]interface{}, 2)
					err := json.Unmarshal(pk.ResponseData, &data)
					if err != nil {
						return
					}
					servers[conn.IdentityData().XUID] = data[0].(string) + ":" + data[1].(string)
					localLoop(conn)
					continue
				}
			case *packet.CommandRequest:
				if pk.CommandLine[0] == '/' {
					if pk.CommandLine[1:] == "proxyhelp" {
						//send help menu
					}
					if pk.CommandLine[1:] == "connect" {
						data := utils.CreateCustomForm("Connect to a server", utils.CustomFormField{Type: "input", Text: "IP",
							PlaceHolder: "", Default: ""}, utils.CustomFormField{Type: "input", Text: "Port", Default: "19132",
							PlaceHolder: ""})
						err := conn.WritePacket(&packet.ModalFormRequest{FormID: 69696969, FormData: data})
						if err != nil {
							panic(err)
						}
						continue
					}
				}
			}
			if err := serverConn.WritePacket(pk); err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = listener.Disconnect(conn, disconnect.Error())
				}
				return
			}
		}
	}()
	go func() {
		defer serverConn.Close()
		defer listener.Disconnect(conn, "connection lost")
		for {
			pk, err := serverConn.ReadPacket()
			if err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = listener.Disconnect(conn, disconnect.Error())
				}
				return
			}
			if err := conn.WritePacket(pk); err != nil {
				return
			}
		}
	}()
}