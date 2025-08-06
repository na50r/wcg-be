package sse

// Base SSE template

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type Message struct {
	Data interface{} `json:"data"`
}

type Broker struct {
	cnt            int
	newClients     chan Subscription
	goneClients    chan Subscription
	ClientChannels map[int]chan []byte

	OnNewClient      func(sub Subscription)
	OnRemoveClient   func(sub Subscription)
	MakeSubscription func(r *http.Request, id int, channel chan []byte) Subscription
}

type Subscription interface {
	GetChannelID() int
	GetChannel() chan []byte
}

// Base subscription
type BaseSubscription struct {
	ChannelID int         `json:"channelId"`
	Channel   chan []byte `json:"channel"`
}

func (s BaseSubscription) GetChannelID() int       { return s.ChannelID }
func (s BaseSubscription) GetChannel() chan []byte { return s.Channel }

func NewSubscription(id int, channel chan []byte) BaseSubscription {
	return BaseSubscription{ChannelID: id, Channel: channel}
}

func NewBroker(
	onNew func(sub Subscription),
	onRemove func(sub Subscription),
	makeSub func(r *http.Request, id int, ch chan []byte) Subscription,
) *Broker {
	b := Broker{
		ClientChannels:   make(map[int]chan []byte),
		newClients:       make(chan Subscription),
		goneClients:      make(chan Subscription),
		cnt:              0,
		OnNewClient:      onNew,
		OnRemoveClient:   onRemove,
		MakeSubscription: makeSub,
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
			log.Printf("client connected (ch=%04b), total clients: %d\n", sub.GetChannelID(), len(b.ClientChannels))
			if b.OnNewClient != nil {
				b.OnNewClient(sub)
			}
		case unsub := <-b.goneClients:
			channel := b.ClientChannels[unsub.GetChannelID()]
			delete(b.ClientChannels, unsub.GetChannelID())
			close(channel)
			if b.OnRemoveClient != nil {
				b.OnRemoveClient(unsub)
			}
			log.Printf("client %04b disconnected, total clients: %d\n", unsub.GetChannelID(), len(b.ClientChannels))
		}
	}
}

func (b *Broker) SSEHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	channelID, channel := b.CreateChannel()
	var sub Subscription
	if b.MakeSubscription != nil {
		sub = b.MakeSubscription(r, channelID, channel)
	} else {
		sub = NewSubscription(channelID, channel)
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

func (b *Broker) PublishToGroup(group map[int]bool, msg Message) {
	data, err := json.Marshal(msg.Data)
	if err != nil {
		log.Printf("unable to marshal: %s", err.Error())
		return
	}
	for cli := range group {
		b.ClientChannels[cli] <- data
	}
}

func (b *Broker) PublishToClient(client int, msg Message) {
	data, err := json.Marshal(msg.Data)
	if err != nil {
		log.Printf("unable to marshal: %s", err.Error())
		return
	}
	b.ClientChannels[client] <- data
}
