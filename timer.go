package main

import (
	"context"
	"fmt"
	"log"
	"time"
	"github.com/na50r/wombo-combo-go-be/sse"
)

type Timer struct {
	durationMinutes int
	cancelFunc      context.CancelFunc
}

func NewTimer(durationMinutes int) *Timer {
	return &Timer{durationMinutes: durationMinutes}
}

func (mt *Timer) Start(s *APIServer, lobbyCode string, game *Game) error {
	if mt.durationMinutes < 1 {
		return fmt.Errorf("duration must be at least 1 minute")
	}
	if mt.durationMinutes >= 5 {
		return fmt.Errorf("duration must be less than or equal to 5 minutes")
	}
	ctx, cancel := context.WithCancel(context.Background())
	mt.cancelFunc = cancel
	ticker := time.NewTicker(time.Second)
	total := mt.durationMinutes * 60
	half := total / 2
	one_quarter := half / 2
	three_quarter := half + one_quarter
	now := time.Now()
	triggers := map[int]bool{three_quarter: false, half: false, one_quarter: false}
	group := s.broker.lobbyClients[lobbyCode]
	publishTimeEvent := func(secondsLeft int) {
		s.broker.Broker.PublishToGroup(group, sse.Message{Data: TimeEvent{SecondsLeft: secondsLeft}})
	}
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				s.broker.Broker.PublishToGroup(group, sse.Message{Data: TIMER_STOPPED})
				log.Printf("Timer %s stopped\n", lobbyCode)
				return
			case t := <-ticker.C:
				elapsed := int(t.Sub(now).Seconds())
				secondsLeft := total - elapsed
				log.Printf("Timer %s: %ds left\n", lobbyCode, secondsLeft)
				switch {
				case secondsLeft <= three_quarter && triggers[three_quarter] == false:
					triggers[three_quarter] = true
					publishTimeEvent(secondsLeft)
				case secondsLeft <= half && triggers[half] == false:
					triggers[half] = true
					publishTimeEvent(secondsLeft)
				case secondsLeft <= one_quarter && triggers[one_quarter] == false:
					triggers[one_quarter] = true
					publishTimeEvent(secondsLeft)
				case secondsLeft <= 10 && secondsLeft > 0:
					publishTimeEvent(secondsLeft)
				case secondsLeft <= 0:
					var err error
					game.Winner, err = s.store.SelectWinnerByPoints(lobbyCode)
					if err != nil {
						log.Printf("Error selecting winner: %v", err)
					}
					s.broker.Broker.PublishToGroup(group, sse.Message{Data: GAME_OVER})
					return
				}
			}
		}
	}()
	return nil
}

func (mt *Timer) Stop() {
	mt.cancelFunc()
}
