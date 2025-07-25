package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
	"unicode"

	"github.com/gorilla/mux"

	"math/rand"

	"context"
	jwt "github.com/golang-jwt/jwt"
)

func getUsername(r *http.Request) (string, error) {
	username := mux.Vars(r)["username"]
	return username, nil
}

func getLobbyCode(r *http.Request) (string, error) {
	lobbyCode := mux.Vars(r)["lobbyCode"]
	return lobbyCode, nil
}

func getPlayername(r *http.Request) (string, error) {
	playerName := mux.Vars(r)["playerName"]
	return playerName, nil
}

func createJWT(account *Account) (string, error) {
	claims := &jwt.MapClaims{
		"exp":      time.Now().Add(time.Hour * 12).Unix(),
		"username": account.Username,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(JWT_SECRET))
}

func createLobbyToken(player *Player) (string, error) {
	claims := &jwt.MapClaims{
		"exp":        time.Now().Add(time.Hour * 4).Unix(),
		"playerName": player.Name,
		"lobbyCode":  player.LobbyCode,
		"hasAccount": player.HasAccount,
		"isOwner":    player.IsOwner,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(JWT_SECRET))
}

func parseJWT(tokenString string) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(JWT_SECRET), nil
	})
}

func getToken(r *http.Request) (jwt.Token, bool, error) {
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		// Anon player
		return jwt.Token{}, false, nil
	}

	token, err := parseJWT(tokenString)
	if err != nil && token != nil && !token.Valid {
		return jwt.Token{}, true, fmt.Errorf("unauthorized")
	}
	return *token, true, nil
}

func passwordValid(password string) error {
	if len(password) < 2 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if len(password) > 20 {
		return fmt.Errorf("password must be less than 20 characters")
	}
	if !IsLetter(password) {
		return fmt.Errorf("password must contain a letter")
	}
	return nil
}

func IsLetter(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func RadixHash(s string, size int) int {
	result := ""
	for _, ch := range s {
		result += strconv.Itoa(int(ch))
	}
	resultInt, _ := strconv.Atoi(result)
	hash := resultInt % size
	return hash
}

func (g *Game) SetTarget() (string, error) {
	if g.GameMode == "Vanilla" {
		return "", nil
	}
	if g.GameMode == "Wombo Combo" {
		log.Println("Number of target words ", len(g.TargetWords))
		targetWord := g.TargetWords[rand.Intn(len(g.TargetWords))]
		return targetWord, nil
	}
	if g.GameMode == "Fusion Frenzy" {
		return g.TargetWord, nil
	}
	return "", fmt.Errorf("game mode %s not found", g.GameMode)
}

// End game if target word is reached (for Wombo Combo and Fusion Frenzy)
// If Game is Vanilla, Game End has to be triggered manually
func (g *Game) EndGame(targetWord, result string) bool {
	if g.GameMode == "Wombo Combo" || g.GameMode == "Fusion Frenzy" {
		return targetWord == result
	}
	if g.GameMode == "Vanilla" {
		return false
	}
	return false
}

func (g *Game) StartTimer(s *APIServer) {
	if g.WithTimer {
		g.Timer.Start(s, g.LobbyCode)
	}
}

func (g *Game) StopTimer() {
	if g.WithTimer {
		g.Timer.Stop()
	}
}

func (mt *MyTimer) Start(s *APIServer, lobbyCode string) error {
	if mt.durationMinutes == 0 {
		return nil
	}
	if mt.durationMinutes >= 5 {
		return fmt.Errorf("duration must be less than or equal to 5 minutes")
	}
	ctx, cancel := context.WithCancel(context.Background())
	mt.cancelFunc = cancel
	ticker := time.NewTicker(time.Second)
	total_duration := time.Duration(mt.durationMinutes) * 60 * time.Second
	half_duration := total_duration / 2
	quarter_duration := half_duration / 2
	three_quarter_duration := half_duration + quarter_duration
	now := time.Now()
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				s.PublishToLobby(lobbyCode, Message{Data: "TIMER_STOPPED"})
				log.Printf("Timer %s stopped\n", lobbyCode)
				return
			case t := <-ticker.C:
				elapsed := t.Sub(now)
				secondsLeft := int((total_duration - elapsed).Seconds())
				s.Publish(Message{Data: "TICK"})
				log.Printf("Timer %s: %ds left\n", lobbyCode, secondsLeft)
				switch {
				case elapsed >= quarter_duration && elapsed <= quarter_duration:
					s.PublishToLobby(lobbyCode, Message{Data: NewTimeEvent(secondsLeft)})
				case elapsed >= half_duration && elapsed <= half_duration:
					s.PublishToLobby(lobbyCode, Message{Data: NewTimeEvent(secondsLeft)})
				case elapsed >= three_quarter_duration && elapsed <= three_quarter_duration:
					s.PublishToLobby(lobbyCode, Message{Data: NewTimeEvent(secondsLeft)})
				case elapsed >= total_duration-10*time.Second && elapsed <= total_duration:
					s.PublishToLobby(lobbyCode, Message{Data: NewTimeEvent(secondsLeft)})
				case elapsed >= total_duration:
					s.PublishToLobby(lobbyCode, Message{Data: "GAME_OVER"})
					return
				}
			}
		}
	}()
	return nil
}

func (mt *MyTimer) Stop() {
	mt.cancelFunc()
}
