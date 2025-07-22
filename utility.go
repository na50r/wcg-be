package main

import (
	"fmt"
	"net/http"
	"time"
	"unicode"
	"strconv"
	"github.com/gorilla/mux"

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