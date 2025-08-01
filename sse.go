package main

// Original based on: https://github.com/plutov/packagemain/tree/master/30-sse
// YouTube video: https://www.youtube.com/watch?v=nvijc5J-JAQ

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	jwt "github.com/golang-jwt/jwt"
)

type Message struct {
	Data interface{} `json:"data"`
}

type Broker struct {
	cnt            int
	newClients     chan Subscription
	goneClients    chan Subscription
	ClientChannels map[int]chan []byte
}

type Subscription struct {
	ChannelID int    `json:"channelId"`
	Channel   chan []byte `json:"channel"`
	LobbyCode string `json:"lobbyCode"`
	PlayerName string `json:"playerName"`
	IsPlayer bool   `json:"isPlayer"`
}

func NewBroker() *Broker {
	b := Broker{
		ClientChannels: make(map[int]chan []byte),
		newClients:     make(chan Subscription),
		goneClients:    make(chan Subscription),
		cnt:            0,
	}
	return &b
}

func (b* Broker) CreateChannel() (int, chan []byte) {
	b.cnt++
	b.ClientChannels[b.cnt] = make(chan []byte)
	return b.cnt, b.ClientChannels[b.cnt]
}

func (s *APIServer) listen() {
	b := s.broker
	for {
		select {
		case sub := <-b.newClients:
			log.Printf("client connected (ch=%04b), total clients: %d\n", sub.ChannelID, len(b.ClientChannels))
			if sub.IsPlayer {
				if s.lobbyClients[sub.LobbyCode] == nil {
					s.lobbyClients[sub.LobbyCode] = make(map[int]bool)
				}
				s.lobbyClients[sub.LobbyCode][sub.ChannelID] = true
				s.playerClient[sub.PlayerName] = sub.ChannelID
				log.Printf("player %s (ch=%04b) connected to lobby %s", sub.PlayerName, sub.ChannelID, sub.LobbyCode)
			}
		case unsub := <-b.goneClients:
			channel := b.ClientChannels[unsub.ChannelID]
			delete(b.ClientChannels, unsub.ChannelID)
			close(channel)
			if unsub.IsPlayer {
				delete(s.lobbyClients[unsub.LobbyCode], unsub.ChannelID)
				delete(s.playerClient, unsub.PlayerName)
				log.Printf("player %s (ch=%04b) disconnected from lobby %s", unsub.PlayerName, unsub.ChannelID, unsub.LobbyCode)
			}
			log.Printf("client %04b disconnected, total clients: %d\n", unsub.ChannelID, len(b.ClientChannels))
		}
	}
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
func (s *APIServer) SSEHandler(w http.ResponseWriter, r *http.Request) {
	b := s.broker
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	token, tokenExists, err := getToken(r)
	log.Printf("Token exists: %v", tokenExists)
	if err != nil {
		return
	}
	var lobbyCode string
	var playerName string
	channelID, channel := b.CreateChannel()
	var sub Subscription
	sub.ChannelID = channelID
	sub.Channel = channel
	sub.IsPlayer = false
	if tokenExists {
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return
		}
		if claims["type"] == "player" {
			lobbyCode = claims["lobbyCode"].(string)
			playerName = claims["playerName"].(string)
			sub.LobbyCode = lobbyCode
			sub.PlayerName = playerName
			sub.IsPlayer = true
		}
	}
	b.newClients <- sub
	clientGone := r.Context().Done()
	rc := http.NewResponseController(w)

	for {
		select {
		case <-clientGone:
			b.goneClients <- sub
			return
		case data := <-channel:
			written, err := fmt.Fprintf(w, "event:msg\ndata:%s\n\n", data)
			log.Printf("written %d bytes", written)
			if err != nil {
				log.Printf("unable to write: %s", err.Error())
				return
			}
			err = rc.Flush()
			if err != nil {
				log.Printf("unable to flush: %s", err.Error())
				return
			}
		}
	}
}

func (s *APIServer) Publish(msg Message) {
	b := s.broker
	data, err := json.Marshal(msg.Data)
	if err != nil {
		log.Printf("unable to marshal: %s", err.Error())
		return
	}
	// Publish to all channels
	// NOTE: Not concurrent
	for _, channel := range b.ClientChannels {
		channel <- data
	}
}

func (s *APIServer) PublishToLobby(lobbyCode string, msg Message) {
	b := s.broker
	clients := s.lobbyClients[lobbyCode]
	data, err := json.Marshal(msg.Data)
	if err != nil {
		log.Printf("unable to marshal: %s", err.Error())
		return
	}
	for cli := range clients {
		b.ClientChannels[cli] <- data
	}
}

func (s *APIServer) PublishToPlayer(playerName string, msg Message) {
	b := s.broker
	channelID := s.playerClient[playerName]
	data, err := json.Marshal(msg.Data)
	if err != nil {
		log.Printf("unable to marshal: %s", err.Error())
		return
	}
	b.ClientChannels[channelID] <- data
}

func (s *APIServer) PublishToChannel(w http.ResponseWriter, r *http.Request) {
	b := s.broker
	channelID, err := getChannelID(r)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var m Message
	err = json.NewDecoder(r.Body).Decode(&m)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	data, err := json.Marshal(m.Data)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	b.ClientChannels[channelID] <- data
}


func (s *APIServer) Broadcast(w http.ResponseWriter, r *http.Request) {
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
	s.Publish(m)
	w.Write([]byte("Msg sent\n"))
}
