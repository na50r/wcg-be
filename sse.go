package main

// Original based on: https://github.com/plutov/packagemain/tree/master/30-sse
// YouTube video: https://www.youtube.com/watch?v=nvijc5J-JAQ

import (
	"encoding/json"
	"fmt"
	jwt "github.com/golang-jwt/jwt"
	"log"
	"net/http"
)

type Message struct {
	Data interface{} `json:"data"`
}

type Broker struct {
	cnt            int
	ClientChannels map[int]chan []byte
}

func NewBroker() *Broker {
	return &Broker{
		ClientChannels: make(map[int]chan []byte),
		cnt:            0,
	}
}

func (b *Broker) CreateChannel() int {
	b.cnt++
	b.ClientChannels[b.cnt] = make(chan []byte)
	return b.cnt
}

func (s *APIServer) SSEHandler(w http.ResponseWriter, r *http.Request) {
	b := s.broker
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	token, tokenExists, err := getToken(r)
	if err != nil {
		return
	}

	channelID := b.CreateChannel()
	channel := b.ClientChannels[channelID]
	if tokenExists {
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return
		}
		lobbyCode := claims["lobbyCode"].(string)
		playerName := claims["playerName"].(string)
		log.Println("Player connected: ", playerName, "to lobby: ", lobbyCode)
		s.players2clients[playerName] = channelID
		if s.lobbies2clients[lobbyCode] == nil {
			s.lobbies2clients[lobbyCode] = make(map[int]bool)
		}
		s.lobbies2clients[lobbyCode][channelID] = true
	}

	fmt.Printf("client connected (id=%d), total clients: %d\n", channelID, len(b.ClientChannels))

	defer func() {
		delete(b.ClientChannels, channelID)
	}()

	clientGone := r.Context().Done()

	rc := http.NewResponseController(w)

	for {
		select {
		case <-clientGone:
			fmt.Printf("client has disconnected (id=%d), total clients: %d\n", channelID, len(b.ClientChannels))
			return
		case data := <-channel:
			if _, err := fmt.Fprintf(w, "event:msg\ndata:%s\n\n", data); err != nil {
				log.Printf("unable to write: %s", err.Error())
				return
			}
			rc.Flush()
		}
	}
}

func (b *Broker) Publish(msg Message) {
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

func (s *APIServer) PublishToClients(lobbyCode string, msg Message) {
	b := s.broker
	clients := s.lobbies2clients[lobbyCode]
	data, err := json.Marshal(msg.Data)
	if err != nil {
		log.Printf("unable to marshal: %s", err.Error())
		return
	}
	for cli := range clients {
		b.ClientChannels[cli] <- data
	}
}

func (b *Broker) PublishEndpoint(w http.ResponseWriter, r *http.Request) {
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
	b.Publish(m)
	w.Write([]byte("Msg sent\n"))
}
