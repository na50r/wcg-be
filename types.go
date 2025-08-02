package main

import (
	"net/http"
	"time"
	"golang.org/x/crypto/bcrypt"
)

type CohereResponse struct {
	ID      string `json:"id"`
	Message struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
}

type APIFunc func(http.ResponseWriter, *http.Request) error

type APIError struct {
	Error string `json:"error"`
}

type GenericResponse struct {
	Message string `json:"message"`
}

type Account struct {
	Username  string `db:"username"`
	Password  string `db:"password"`
	Wins      int    `db:"wins"`
	Losses    int    `db:"losses"`
	ImageName string `db:"image_name"`
	CreatedAt string `db:"created_at"`
	Status    Status `db:"status"`
	IsOwner   bool   `db:"is_owner"`
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
	Username  string `json:"username"`  // Username of the account
	Wins      int    `json:"wins"`      // Number of wins
	Losses    int    `json:"losses"`    // Number of losses
	ImageName string `json:"imageName"` // Name of the user's profile image
	Image     []byte `json:"image"`     // Base64-encoded image
	CreatedAt string `json:"createdAt"` // ISO8601 creation timestamp
	Status    Status `json:"status"`    // ONLINE or OFFLINE
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
	LobbyCode  string `db:"lobby_code"`
	Name       string `db:"name"`
	ImageName  string `db:"image_name"`
	IsOwner    bool   `db:"is_owner"`
	HasAccount bool   `db:"has_account"`
	TargetWord string `db:"target_word"`
	Points     int    `db:"points"`
}

type Lobby struct {
	Name        string `db:"name"`
	ImageName   string `db:"image_name"`
	LobbyCode   string `db:"lobby_code"`
	GameMode    GameMode `db:"game_mode"`
	PlayerCount int    `db:"player_count"`
}

type PlayerDTO struct {
	Name  string `json:"name"`
	Image []byte `json:"image"`
}

type LobbyDTO struct {
	LobbyCode string       `json:"lobbyCode"`
	Name      string       `json:"name"`
	GameMode  GameMode       `json:"gameMode"`
	Owner     string       `json:"owner"`
	Players   []*PlayerDTO `json:"players"`
	GameModes []GameMode   `json:"gameModes"`
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
	Name string `db:"name"`
	Data []byte `db:"data"`
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

type EditGameRequest struct {
	GameMode GameMode `json:"gameMode"`
	Duration int    `json:"duration"`
}

type GameEditEvent struct {
	GameMode GameMode `json:"gameMode"`
	Duration int    `json:"duration"`
}

type Game struct {
	GameMode    GameMode   `json:"gameMode"`
	LobbyCode   string   `json:"lobbyCode"`
	TargetWord  string   `json:"targetWord"`
	TargetWords []string `json:"targetWords"`
	Winner      string   `json:"winner"`
	WithTimer   bool     `json:"withTimer"`
	Timer       *Timer   `json:"timer"`
}

type Combination struct {
	A      string `db:"a"`
	B      string `db:"b"`
	Result string `db:"result"`
	Depth  int    `db:"depth"`
}

type Word struct {
	Word         string  `db:"word"`
	Depth        int     `db:"depth"`
	Reachability float64 `db:"reachability"`
}

type WordRequest struct {
	A string `json:"a"`
	B string `json:"b"`
}

type WordResponse struct {
	Result string `json:"result"`
	IsNew  bool   `json:"isNew"`
}

type Words struct {
	Words      []string `json:"words"`
	TargetWord string   `json:"targetWord"`
}

type StartGameRequest struct {
	GameMode  GameMode `json:"gameMode"`
	WithTimer bool   `json:"withTimer"`
	Duration  int    `json:"duration"`
}

type PlayerWord struct {
	PlayerName string `db:"player_name"`
	Word       string `db:"word"`
	LobbyCode  string `db:"lobby_code"`
	Timestamp  string `db:"timestamp"`
}

type PlayerWordCount struct {
	PlayerName string `json:"playerName"`
	WordCount  int    `json:"wordCount"`
}

type PlayerResultDTO struct {
	PlayerName string `json:"playerName"`
	Image      []byte `json:"image"`
	WordCount  int    `json:"wordCount"`
	Points     int    `json:"points"`
}

type GameEndResponse struct {
	GameMode    GameMode             `json:"gameMode"`
	Winner      string             `json:"winner"`
	PlayerWords []*PlayerResultDTO `json:"playerResults"`
}

type TimeEvent struct {
	SecondsLeft int `json:"secondsLeft"`
}

type ChallengeEntry struct {
	Timestamp time.Time `db:"timestamp"`
	WordCount int `db:"word_count"`
	Username string `db:"username"`
}

type ChallengeEntryDTO struct {
	WordCount int `json:"wordCount"`
	Username string `json:"username"`
	Image []byte `json:"image"`
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
		Status:    OFFLINE,
		IsOwner:   false,
	}, nil
}

func NewPlayer(name, lobbyCode, imageName string, isOwner, hasAccount bool) *Player {
	return &Player{
		LobbyCode:  lobbyCode,
		Name:       name,
		ImageName:  imageName,
		IsOwner:    isOwner,
		HasAccount: hasAccount,
		TargetWord: "",
		Points:     0,
	}
}

func NewGameModes() []GameMode {
	return []GameMode{VANILLA, WOMBO_COMBO, FUSION_FRENZY, DAILY_CHALLENGE}
}

func NewLobby(name, lobbyCode, imageName string) *Lobby {
	return &Lobby{
		Name:        name,
		ImageName:   imageName,
		LobbyCode:   lobbyCode,
		GameMode:    VANILLA,
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
		GameModes: NewGameModes(),
	}
}

func NewTimer(durationMinutes int) *Timer {
	return &Timer{durationMinutes: durationMinutes}
}
