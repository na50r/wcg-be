package main

import (
	"database/sql"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Storage interface {
	Init() error
	CreateAccount(acc *Account) error
	CreatePlayer(player *Player) error
	CreateLobby(lobby *Lobby) error
	DeleteAccount(username string) error
	GetPlayerByLobbyCodeAndName(name, lobbyCode string) (*Player, error)
	DeletePlayer(name, lobbyCode string) error
	GetPlayersByLobbyCode(lobbyCode string) ([]*Player, error)
	GetAccountByUsername(username string) (*Account, error)
	UpdateAccount(acc *Account) error
	AddImage(data []byte, name string) error
	GetImage(name string) ([]byte, error)
	GetImages() ([]*Image, error)
	NewImageForUsername(username string) string
	GetPlayerForAccount(username string) (*Player, error)
	GetLobbyForOwner(owner string) (string, error)
	DeletePlayersForLobby(lobbyCode string) error
	AddPlayerToLobby(lobbyCode string, player *Player) error
	DeleteLobby(lobbyCode string) error
	GetLobbies() ([]*Lobby, error)
	GetLobbyByCode(lobbyCode string) (*Lobby, error)
	EditGameMode(lobbyCode string, gameMode GameMode) error
	AddCombination(element *Combination) error
	GetCombination(a, b string) (*string, bool, error)
	AddWord(word *Word) error
	AddPlayerWord(playerName, word, lobbyCode string) error
	GetPlayerWords(playerName, lobbyCode string) ([]string, error)
	DeletePlayerWordsByLobbyCode(lobbyCode string) error
	DeletePlayerWordsByPlayerAndLobbyCode(playerName, lobbyCode string) error
	GetWordCountByLobbyCode(lobbyCode string) ([]*PlayerWordCount, error)
	UpdateAccountWinsAndLosses(lobbyCode, winner string) error
	SetPlayerTargetWord(playerName, targetWord, lobbyCode string) error
	GetPlayerTargetWord(playerName, lobbyCode string) (string, error)
	IsPlayerWord(playerName, word, lobbyCode string) (bool, error)
	IncrementPlayerPoints(playerName, lobbyCode string, points int) error
	SetIsOwner(username string, setOwner bool) error
	SelectWinnerByPoints(lobbyCode string) (string, error)
	ResetPlayerPoints(lobbyCode string) error
	IncrementPlayerCount(lobbyCode string, increment int) error
	AddNewCombination(a, b, result string) error
	CreateOrGetDailyWord(minReachability, maxReachability float64, maxDepth int) (string, error)
	AddDailyChallengeEntry(wordCount int, username string) error
	GetChallengeEntries() ([]*Challenger, error)
	GetImageByUsername(username string) ([]byte, error)
	GetTargetWords(minReachability, maxReachability float64, maxDepth int) ([]string, error)
	GetTargetWord(minReachability, maxReachability float64, maxDepth int) (string, error)
}

// DB Types
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
	Name        string   `db:"name"`
	ImageName   string   `db:"image_name"`
	LobbyCode   string   `db:"lobby_code"`
	GameMode    GameMode `db:"game_mode"`
	PlayerCount int      `db:"player_count"`
}

type Image struct {
	Name string `db:"name"`
	Data []byte `db:"data"`
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

type PlayerWord struct {
	PlayerName string `db:"player_name"`
	Word       string `db:"word"`
	LobbyCode  string `db:"lobby_code"`
	Timestamp  string `db:"timestamp"`
}

type Challenger struct {
	Timestamp time.Time `db:"timestamp"`
	WordCount int       `db:"word_count"`
	Username  string    `db:"username"`
}

type Session struct {
	ID           string    `db:"id"`
	RefreshToken string    `db:"refresh_token"`
	Username     string    `db:"username"`
	IsRevoked    bool      `db:"is_revoked"`
	CreatedAt    time.Time `db:"created_at"`
	ExpiresAt    time.Time `db:"expires_at"`
}

// Constructors
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

func NewLobby(name, lobbyCode, imageName string) *Lobby {
	return &Lobby{
		Name:        name,
		ImageName:   imageName,
		LobbyCode:   lobbyCode,
		GameMode:    VANILLA,
		PlayerCount: 1,
	}
}

// Convert SQL rows into an defined Go types
func scanIntoAccount(rows *sql.Rows) (*Account, error) {
	acc := new(Account)
	err := rows.Scan(
		&acc.Username,
		&acc.ImageName,
		&acc.Password,
		&acc.Wins,
		&acc.Losses,
		&acc.CreatedAt,
		&acc.Status,
		&acc.IsOwner,
	)
	return acc, err
}

func scanIntoImage(rows *sql.Rows) (*Image, error) {
	image := new(Image)
	err := rows.Scan(
		&image.Name,
		&image.Data,
	)
	return image, err
}

func scanIntoPlayer(rows *sql.Rows) (*Player, error) {
	player := new(Player)
	err := rows.Scan(
		&player.Name,
		&player.LobbyCode,
		&player.ImageName,
		&player.IsOwner,
		&player.HasAccount,
		&player.TargetWord,
		&player.Points,
	)
	return player, err
}

func scanIntoLobby(rows *sql.Rows) (*Lobby, error) {
	lobby := new(Lobby)
	err := rows.Scan(
		&lobby.Name,
		&lobby.ImageName,
		&lobby.LobbyCode,
		&lobby.GameMode,
		&lobby.PlayerCount,
	)
	return lobby, err
}

func scanIntoWord(rows *sql.Rows) (*Word, error) {
	word := new(Word)
	err := rows.Scan(
		&word.Word,
		&word.Depth,
		&word.Reachability,
	)
	return word, err
}

func scanIntoPlayerWord(rows *sql.Rows) (*PlayerWord, error) {
	playerWord := new(PlayerWord)
	err := rows.Scan(
		&playerWord.PlayerName,
		&playerWord.Word,
		&playerWord.LobbyCode,
		&playerWord.Timestamp,
	)
	return playerWord, err
}

func scanIntoPlayerWordCount(rows *sql.Rows) (*PlayerWordCount, error) {
	wordCount := new(PlayerWordCount)
	err := rows.Scan(
		&wordCount.PlayerName,
		&wordCount.WordCount,
	)
	return wordCount, err
}

func scanIntoChallengeEntry(rows *sql.Rows) (*Challenger, error) {
	entry := new(Challenger)
	err := rows.Scan(
		&entry.Timestamp,
		&entry.WordCount,
		&entry.Username,
	)
	return entry, err
}

func scanIntoSession(rows *sql.Rows) (*Session, error) {
	session := new(Session)
	err := rows.Scan(
		&session.ID,
		&session.RefreshToken,
		&session.Username,
		&session.IsRevoked,
		&session.CreatedAt,
		&session.ExpiresAt,
	)
	return session, err
}
