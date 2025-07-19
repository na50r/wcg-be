package main

import (
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type APIFunc func(http.ResponseWriter, *http.Request) error

type APIError struct {
	Error string `json:"error"`
}

type GenericResponse struct {
	Message string `json:"message"`
}

type Account struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Wins      int    `json:"wins"`
	Losses    int    `json:"losses"`
	ImageName string `json:"imageName"`
	CreatedAt string `json:"createdAt"`
	Status    string `json:"status"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type AccountDTO struct {
	Username  string `json:"username"`
	Wins      int    `json:"wins"`
	Losses    int    `json:"losses"`
	ImageName string `json:"imageName"`
	Image     []byte `json:"image"`
	CreatedAt string `json:"createdAt"`
	Status    string `json:"status"`
}

type CreateLobbyRequest struct {
	Name string `json:"name"`
}

type Player struct {
	LobbyID   string `json:"lobbyID"`
	Name      string `json:"name"`
	ImageName string `json:"imageName"`
	IsOwner   bool   `json:"isOwner"`
	HasAccount bool   `json:"hasAccount"`
}

type PlayerDTO struct {
	Name  string `json:"name"`
	Image []byte `json:"image"`
}

type LobbyDTO struct {
	Owner   PlayerDTO    `json:"owner"`
	Players []*PlayerDTO `json:"players"`
}

type LobbiesDTO struct {
	Owner PlayerDTO    `json:"owner"`
	PlayerCount int      `json:"playerCount"`
	LobbyID string     `json:"lobbyID"`
}

type UpdateAccountRequest struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	ImageName string `json:"imageName"`
}

type Image struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

type ImagesResponse struct {
	Names []string `json:"names"`
}

type ChangeImageRequest struct {
	ImageName string `json:"imageName"`
}

type JoinLobbyRequest struct {
	Name    string `json:"name"`
	LobbyID string `json:"lobbyID"`
}

func NewAccount(username, password string) (*Account, error) {
	encpw, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	imageName := "default.png"
	return &Account{
		Username:  username,
		Password:  string(encpw),
		Wins:      0,
		Losses:    0,
		ImageName: imageName,
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
		Status:    "OFFLINE",
	}, nil
}

func NewPlayer(name, lobbyID, imageName string, isOwner bool) *Player {
	return &Player{
		LobbyID:   lobbyID,
		Name:      name,
		ImageName: imageName,
		IsOwner:   isOwner,
	}
}
