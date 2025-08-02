package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"context"
)

func getToken(r *http.Request) (string, bool) {
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		// Anon player
		return "", false
	}
	tokenString = strings.Replace(tokenString, "Bearer ", "", 1)
	return tokenString, true
}

func verifyAccountJWT(tokenString string) (*AccountClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AccountClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(JWT_SECRET), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*AccountClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

func verifyPlayerJWT(tokenString string) (*PlayerClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &PlayerClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(JWT_SECRET), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*PlayerClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

func createJWT(account *Account) (string, error) {
	claims, err := NewAccountClaims(account.Username, time.Hour*4)
	if err != nil {
		return "", err
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(JWT_SECRET))
}

func createLobbyToken(player *Player) (string, error) {
	claims, err := NewPlayerClaims(player, time.Hour*4)
	if err != nil {
		return "", err
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(JWT_SECRET))
}

type AccountClaims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

func NewAccountClaims(username string, duration time.Duration) (*AccountClaims, error) {
	tokenID, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token ID: %v", err)
	}
	return &AccountClaims{
		Username: username,
		StandardClaims: jwt.StandardClaims{
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(duration).Unix(),
			Id:        tokenID.String(),
			Subject:   "account",
		},
	}, nil
}

type PlayerClaims struct {
	PlayerName string `json:"playerName"`
	LobbyCode  string `json:"lobbyCode"`
	HasAccount bool   `json:"hasAccount"`
	IsOwner    bool   `json:"isOwner"`
	jwt.StandardClaims
}

func NewPlayerClaims(player *Player, duration time.Duration) (*PlayerClaims, error) {
	tokenID, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token ID: %v", err)
	}
	return &PlayerClaims{
		PlayerName: player.Name,
		LobbyCode:  player.LobbyCode,
		HasAccount: player.HasAccount,
		IsOwner:    player.IsOwner,
		StandardClaims: jwt.StandardClaims{
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(duration).Unix(),
			Id:        tokenID.String(),
			Subject:   "player",
		},
	}, nil
}

type authKey struct{}

// Authentication Middleware Adapted from Anthony GG's tutorial
func withAccountAuth(handlerFunc http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, tokenExists := getToken(r)
		if !tokenExists {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			return
		}
		accountClaims, err := verifyAccountJWT(token)
		if err != nil {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			return
		}
		username, err := getUsername(r)
		if err != nil {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			return
		}
		if accountClaims.Username != username {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			return
		}
		ctx := context.WithValue(r.Context(), authKey{}, accountClaims)
		r = r.WithContext(ctx)
		handlerFunc(w, r)
	}
}

func withPlayerAuth(handlerFunc http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, tokenExists := getToken(r)
		if !tokenExists {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			log.Println("Unauthorized (No Token)")
			return
		}
		playerClaims, err := verifyPlayerJWT(token)
		if err != nil {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			log.Println("Unauthorized (Invalid Token)", err)
			return
		}
		lobbyCode, err := getLobbyCode(r)
		if err != nil {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			log.Println("Unauthorized (No Lobby Code)", err)
			return
		}
		playerName, err := getPlayername(r)
		if err != nil {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			log.Println("Unauthorized (No Player Name)", err)
			return
		}
		if lobbyCode != playerClaims.LobbyCode || playerName != playerClaims.PlayerName {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			log.Println("Unauthorized (Invalid Lobby Code or Player Name)", err)
			return
		}
		ctx := context.WithValue(r.Context(), authKey{}, playerClaims)
		r = r.WithContext(ctx)
		handlerFunc(w, r)
	}
}
