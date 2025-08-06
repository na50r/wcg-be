package sse

// Base SSE template

import (
	"fmt"
	"log"
	"net/http"
	"encoding/json"
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
	ChannelID  int         `json:"channelId"`
	Channel    chan []byte `json:"channel"`
}

func NewSubscription(id int, channel chan[]byte) Subscription {
	return Subscription{ChannelID: id, Channel: channel}
}

func NewBroker() *Broker {
	b := Broker{
		ClientChannels: make(map[int]chan []byte),
		newClients:     make(chan Subscription),
		goneClients:    make(chan Subscription),
		cnt:            0,
	}
	go b.listen()
	return &b
}

func (b *Broker) CreateChannel() (int, chan []byte) {
	b.cnt++
	b.ClientChannels[b.cnt] = make(chan []byte)
	return b.cnt, b.ClientChannels[b.cnt]
}

func (b *Broker) listen() {
	for {
		select {
		case sub := <-b.newClients:
			log.Printf("client connected (ch=%04b), total clients: %d\n", sub.ChannelID, len(b.ClientChannels))
		case unsub := <-b.goneClients:
			channel := b.ClientChannels[unsub.ChannelID]
			delete(b.ClientChannels, unsub.ChannelID)
			close(channel)
			log.Printf("client %04b disconnected, total clients: %d\n", unsub.ChannelID, len(b.ClientChannels))
		}
	}
}

func (b *Broker) SSEHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	channelID, channel := b.CreateChannel()
	sub := NewSubscription(channelID, channel)

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


func (b *Broker) Publish(msg Message) {
	data, err := json.Marshal(msg.Data)
	if err != nil {
		log.Printf("unable to marshal: %s", err.Error())
		return
	}
	for _, channel := range b.ClientChannels {
		channel <- data
	}
}

func (b *Broker) PublishToGroup(group []int, msg Message){
	data, err := json.Marshal(msg.Data)
	if err != nil {
		log.Printf("unable to marshal: %s", err.Error())
		return
	}
	for cli := range group {
		b.ClientChannels[cli] <- data
	}
}

func (b *Broker) PublishToClient(client int, msg Message){
	data, err := json.Marshal(msg.Data)
	if err != nil {
		log.Printf("unable to marshal: %s", err.Error())
		return
	}
	b.ClientChannels[client] <- data
}

