package main

import (
	"log"
	"net/http"
	"encoding/json"
	"github.com/na50r/wombo-combo-go-be/sse"
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
	sse.BaseSubscription
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

func MakePlayerSubscription(r *http.Request, id int, channel chan []byte) sse.Subscription {
	token, tokenExists := getToken(r)
	ps := PlayerSubscription{
		BaseSubscription: sse.NewSubscription(id, channel),
		IsPlayer:         false,
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

func (gb *GameBroker) OnNewPlayerSub(sub sse.Subscription) {
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

func (gb *GameBroker) OnRemovePlayerSub(unsub sse.Subscription) {
	ps, ok := unsub.(PlayerSubscription)
	if !ok {
		log.Println("Type conversion failed")
		return
	}
	if !ps.IsPlayer {
		return
	}
	delete(gb.lobbyClients[ps.LobbyCode], ps.ChannelID)
	delete(gb.playerClient, ps.PlayerName)
	log.Printf("player %s (ch=%d) disconnected from lobby %s", ps.PlayerName, ps.ChannelID, ps.LobbyCode)
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

// SSEHandler godoc
// @Summary Server-Sent Events
// @Description Server-Sent Events
// @Tags events
// @Accept json
// @Produce json
// @Success 200 {object} Message
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /events [get]
func (gb *GameBroker) SSEHandler(w http.ResponseWriter, r *http.Request) {
	gb.Broker.SSEHandler(w, r)
}

//$ curl -X POST -H "Content-Type: application/json" -d '{"data": "Hello World"}' http://localhost:<port>/broadcast
func (gb *GameBroker) Broadcast(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var m Message
	err := json.NewDecoder(r.Body).Decode(&m)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	gb.Broker.Publish(m.toSSE())
	w.Write([]byte("Msg sent\n"))
}