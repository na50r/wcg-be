package main

import (
	"github.com/na50r/wombo-combo-go-be/sse"
	"log"
	"net/http"
)

type Message struct {
	Data interface{} `json:"data"`
}

func (m *Message) toSSE() sse.Message {
	return sse.Message{
		Data: m.Data,
	}
}

type GameBroker struct {
	Broker       sse.Broker
	lobbyClients map[string]map[int]bool
	playerClient map[string]int
}

type PlayerSubscription struct {
	sse.Subscription
	LobbyCode  string
	PlayerName string
	IsPlayer   bool
}

func NewGameBroker() *GameBroker {
	gb := &GameBroker{
		lobbyClients: make(map[string]map[int]bool),
		playerClient: make(map[string]int),
	}

	gb.Broker = *sse.NewBroker(
		gb.OnNewPlayerSub,
		gb.OnRemovePlayerSub,
		MakePlayerSubscription,
	)
	return gb
}

func (ps PlayerSubscription) GetChannelID() int       { return ps.ChannelID }
func (ps PlayerSubscription) GetChannel() chan []byte { return ps.Channel }

func MakePlayerSubscription(r *http.Request, id int, channel chan []byte) sse.Sub {
	token, tokenExists := getToken(r)
	ps := PlayerSubscription{
		Subscription: sse.NewSubscription(id, channel),
		IsPlayer:     false,
	}
	if !tokenExists {
		return ps
	}
	claims, err := verifyPlayerJWT(token)
	if err != nil {
		log.Printf("JWT verification failed: %v", err)
		return ps
	}
	ps.PlayerName = claims.PlayerName
	ps.LobbyCode = claims.LobbyCode
	ps.IsPlayer = true
	return ps
}

func (gb *GameBroker) OnNewPlayerSub(sub sse.Sub) {
	ps, ok := sub.(PlayerSubscription)
	if !ok {
		log.Println("Type conversion failed")
		return
	}
	if !ps.IsPlayer {
		return
	}
	if gb.lobbyClients[ps.LobbyCode] == nil {
		gb.lobbyClients[ps.LobbyCode] = make(map[int]bool)
	}
	gb.lobbyClients[ps.LobbyCode][ps.ChannelID] = true
	gb.playerClient[ps.PlayerName] = ps.ChannelID
}

func (gb *GameBroker) OnRemovePlayerSub(unsub sse.Sub) {
	ps, ok := unsub.(PlayerSubscription)
	if !ok {
		log.Println("Type conversion failed")
		return
	}
	if !ps.IsPlayer {
		return
	}
	delete(gb.lobbyClients[ps.LobbyCode], unsub.GetChannelID())
	delete(gb.playerClient, ps.PlayerName)
	log.Printf("player %s (ch=%04b) disconnected from lobby %s", ps.PlayerName, ps.ChannelID, ps.LobbyCode)
}

func (gb *GameBroker) PublishToLobby(lobbyCode string, msg Message) {
	group := gb.lobbyClients[lobbyCode]
	gb.Broker.PublishToGroup(group, msg.toSSE())
}

func (gb *GameBroker) PublishToPlayer(playername string, msg Message) {
	cli := gb.playerClient[playername]
	gb.Broker.PublishToClient(cli, msg.toSSE())
}

func (gb *GameBroker) Publish(msg Message) {
	gb.Broker.Publish(msg.toSSE())
}