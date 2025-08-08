package dto

import (
	c "github.com/na50r/wombo-combo-go-be/constants"
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

type APIError struct {
	Error string `json:"error"`
}

type GenericResponse struct {
	Message string `json:"message"`
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
	Status    c.Status `json:"status"`    // ONLINE or OFFLINE
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

type PlayerDTO struct {
	Name  string `json:"name"`
	Image []byte `json:"image"`
}

type LobbyDTO struct {
	LobbyCode string       `json:"lobbyCode"`
	Name      string       `json:"name"`
	GameMode  c.GameMode     `json:"gameMode"`
	Owner     string       `json:"owner"`
	Players   []*PlayerDTO `json:"players"`
	GameModes []c.GameMode   `json:"gameModes"`
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
	GameMode c.GameMode `json:"gameMode"`
	Duration int      `json:"duration"`
}

type GameEditEvent struct {
	GameMode c.GameMode `json:"gameMode"`
	Duration int      `json:"duration"`
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
	GameMode  c.GameMode `json:"gameMode"`
	WithTimer bool     `json:"withTimer"`
	Duration  int      `json:"duration"`
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
	GameMode    c.GameMode           `json:"gameMode"`
	Winner      string             `json:"winner"`
	PlayerWords []*PlayerResultDTO `json:"playerResults"`
	ManualEnd   bool               `json:"manualEnd"`
}

type TimeEvent struct {
	SecondsLeft int `json:"secondsLeft"`
}

type AchievementEvent struct {
	AchievementTitle string `json:"achievementTitle"`
}

type ChallengeEntryDTO struct {
	WordCount int    `json:"wordCount"`
	Username  string `json:"username"`
	Image     []byte `json:"image"`
}

type AchievementDTO struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Image       []byte `json:"image"`
	Unlocked    bool   `json:"unlocked"`
}

func NewGameModes() []c.GameMode {
	return []c.GameMode{c.VANILLA, c.WOMBO_COMBO, c.FUSION_FRENZY, c.DAILY_CHALLENGE}
}

