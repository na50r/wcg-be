package main

import(
	"fmt"
	"log"
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
	ManualEnd   bool     `json:"manualEnd"`
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
	ManualEnd   bool               `json:"manualEnd"`
}

type TimeEvent struct {
	SecondsLeft int `json:"secondsLeft"`
}


type ChallengeEntryDTO struct {
	WordCount int `json:"wordCount"`
	Username string `json:"username"`
	Image []byte `json:"image"`
}

func NewGameModes() []GameMode {
	return []GameMode{VANILLA, WOMBO_COMBO, FUSION_FRENZY, DAILY_CHALLENGE}
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


func NewGame(s Storage, lobbyCode string, gameMode GameMode, withTimer bool, duration int) (*Game, error) {
	game := new(Game)
	game.LobbyCode = lobbyCode
	game.GameMode = gameMode
	game.WithTimer = withTimer
	game.ManualEnd = false

	if withTimer {
		game.Timer = NewTimer(duration)
	}

	err := fmt.Errorf("Game mode %s not found", gameMode)
	if gameMode == VANILLA {
		return game, nil
	}
	// Reachability is between 0 and 1
	// Reachability is computed with: 1 / (2 ^ depth)
	// Reachability is updated with: 
	// 0.75 * newReachability + 0.25 * oldReachability if newDepth < oldDepth
	// 0.25 * newReachability + 0.75 * oldReachability if newDepth >= oldDepth
	// The less deep and the more paths are available, the more reachable a word is
	if gameMode == FUSION_FRENZY {
		game.TargetWord, err = s.GetTargetWord(0.0375, 0.2, 10)
		if err != nil {
			return nil, err
		}
		return game, nil
	}
	if gameMode == WOMBO_COMBO {
		game.TargetWords, err = s.GetTargetWords(0.0375, 0.2, 10)
		if err != nil {
			return nil, err
		}
		return game, nil
	}
	if gameMode == DAILY_CHALLENGE {
		game.TargetWord, err = s.CreateOrGetDailyWord(0.0375, 0.2, 8)
		if err != nil {
			log.Printf("Error creating or getting daily word: %v", err)
			return nil, err
		}
		return game, nil
	}
	return nil, err
}