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

type CreateLobbyResponse struct {
	Token    string `json:"token"`
	LobbyDTO `json:"lobby"`
}

type JoinLobbyRespone struct {
	Token    string `json:"token"`
	LobbyDTO `json:"lobby"`
}

type Player struct {
	LobbyCode  string `json:"lobbyCode"`
	Name       string `json:"name"`
	ImageName  string `json:"imageName"`
	IsOwner    bool   `json:"isOwner"`
	HasAccount bool   `json:"hasAccount"`
}

type Lobby struct {
	Name        string `json:"name"`
	ImageName   string `json:"imageName"`
	LobbyCode   string `json:"lobbyCode"`
	GameMode    string `json:"gameMode"`
	PlayerCount int    `json:"playerCount"`
}

type PlayerDTO struct {
	Name  string `json:"name"`
	Image []byte `json:"image"`
}

type LobbyDTO struct {
	LobbyCode string       `json:"lobbyCode"`
	Name      string       `json:"name"`
	GameMode  string       `json:"gameMode"`
	Owner     string       `json:"owner"`
	Players   []*PlayerDTO `json:"players"`
	GameModes []string     `json:"gameModes"`
}

type LobbiesDTO struct {
	Image       []byte `json:"image"`
	PlayerCount int    `json:"playerCount"`
	LobbyCode   string `json:"lobbyCode"`
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

type EditAccountRequest struct {
	Type        string `json:"type"`
	Username    string `json:"username"`
	ImageName   string `json:"imageName"`
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type JoinLobbyRequest struct {
	PlayerName string `json:"playerName"`
	LobbyCode  string `json:"lobbyCode"`
}

type ChangeGameModeRequest struct {
	GameMode string `json:"gameMode"`
}

type GameModeChangeEvent struct {
	GameMode string `json:"gameMode"`
}

type Game struct {
	LobbyCode  string `json:"lobbyCode"`
	TargetWord string `json:"targetWord"`
}

type Combination struct {
	A      string `json:"a"`
	B      string `json:"b"`
	Result string `json:"result"`
	Depth  int    `json:"depth"`
}

type Word struct {
	Word      string  `json:"word"`
	Depth     int     `json:"depth"`
	Reachability float64 `json:"reachability"`
}

type WordRequest struct {
	A string `json:"a"`
	B string `json:"b"`
}

type WordResponse struct {
	Result string `json:"result"`
}

type StartGameRequest struct {
	GameMode string `json:"gameMode"`
}

type StartGameResponse struct {
	TargetWord string `json:"targetWord"`
}

type PlayerWord struct {
	PlayerName string `json:"playerName"`
	Word       string `json:"word"`
	LobbyCode  string `json:"lobbyCode"`
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

func NewPlayer(name, lobbyCode, imageName string, isOwner, hasAccount bool) *Player {
	return &Player{
		LobbyCode:  lobbyCode,
		Name:       name,
		ImageName:  imageName,
		IsOwner:    isOwner,
		HasAccount: hasAccount,
	}
}

func GameModes() []string {
	return []string{"Vanilla", "Wombo Combo", "Lucky Rush"}
}

func NewLobby(name, lobbyCode, imageName string) *Lobby {
	return &Lobby{
		Name:        name,
		ImageName:   imageName,
		LobbyCode:   lobbyCode,
		GameMode:    "Vanilla",
		PlayerCount: 1,
	}
}

func NewLobbyDTO(lobby *Lobby, owner string, players []*PlayerDTO) *LobbyDTO {
	return &LobbyDTO{
		LobbyCode: lobby.LobbyCode,
		Name:      lobby.Name,
		GameMode:  lobby.GameMode,
		Owner:     owner,
		Players:   players,
		GameModes: GameModes(),
	}
}
