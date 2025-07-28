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

func ProcessMove(server *APIServer, game *Game, player *Player, result string) error {
	if game.GameMode == "Fusion Frenzy" && player.TargetWord == result {
		game.StopTimer()
		game.Winner = player.Name
		if err := server.store.UpdateAccountWinsAndLosses(game.LobbyCode, player.Name); err != nil {
			return err
		}
		server.PublishToLobby(game.LobbyCode, Message{Data: GAME_OVER})
		server.PublishToLobby(game.LobbyCode, Message{Data: ACCOUNT_UPDATE})
		return nil
	}
	if game.GameMode == "Wombo Combo" && player.TargetWord == result {
		var newTargetWord string
		var err error
		for {
			newTargetWord, err = game.SetTarget()
			if err != nil {
				return err
			}
			if newTargetWord != player.TargetWord {
				break
			}
		}
		log.Printf("Player %s reached target word %s, new target word is %s", player.Name, player.TargetWord, newTargetWord)
		if err := server.store.SetPlayerTargetWord(player.Name, newTargetWord, game.LobbyCode); err != nil {
			return err
		}
		if err := server.store.IncrementPlayerPoints(player.Name, game.LobbyCode, 10); err != nil {
			return err
		}
		server.PublishToLobby(game.LobbyCode, Message{Data: WOMBO_COMBO_EVENT})
	}
	isPlayerWord, err := server.store.IsPlayerWord(player.Name, result, game.LobbyCode)
	if err != nil {
		return err
	}
	if isPlayerWord {
		if err := server.store.IncrementPlayerPoints(player.Name, game.LobbyCode, 1); err != nil {
			return err
		}
	}
	if err := server.store.AddPlayerWord(player.Name, result, game.LobbyCode); err != nil {
		return err
	}
	return nil
}

func (g *Game) StartTimer(s *APIServer) {
	if g.WithTimer {
		g.Timer.Start(s, g.LobbyCode, g)
	}
}

func (g *Game) StopTimer() {
	if g.WithTimer {
		g.Timer.Stop()
	}
}

func (mt *Timer) Start(s *APIServer, lobbyCode string, game *Game) error {
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

	publishTimeEvent := func (secondsLeft int) {
		s.PublishToLobby(lobbyCode, Message{Data: TimeEvent{SecondsLeft: secondsLeft}})
	}

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				s.PublishToLobby(lobbyCode, Message{Data: TIMER_STOPPED})
				log.Printf("Timer %s stopped\n", lobbyCode)
				return
			case t := <-ticker.C:
				elapsed := t.Sub(now)
				secondsLeft := int((total_duration - elapsed).Seconds())
				log.Printf("Timer %s: %ds left\n", lobbyCode, secondsLeft)
				switch elapsed {
				case quarter_duration:
					publishTimeEvent(secondsLeft)
				case half_duration:
					publishTimeEvent(secondsLeft)
				case three_quarter_duration:
					publishTimeEvent(secondsLeft)
				case total_duration:
					s.PublishToLobby(lobbyCode, Message{Data: GAME_OVER})
					var err error
					game.Winner, err = s.store.SelectWinnerByPoints(lobbyCode)
					if err != nil {
						log.Printf("Error selecting winner: %v", err)
					}
					return
				default:
					if secondsLeft <= 10 {
					publishTimeEvent(secondsLeft)
					}
				}
			}
		}
	}()
	return nil
}

func (mt *Timer) Stop() {
	mt.cancelFunc()
}
