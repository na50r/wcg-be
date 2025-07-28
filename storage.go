package main

import (
	"database/sql"
)

type Storage interface {
	Init() error
	CreateAccount(acc *Account) error
	CreatePlayer(player *Player) error
	CreateLobby(lobby *Lobby) error
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
	EditGameMode(lobbyCode, gameMode string) error
	AddCombination(element *Combination) error
	GetCombination(a, b string) (*string, error)
	NewGame(lobbyCode string, gameMode string, withTimer bool, duration int) (*Game, error)
	AddWord(word *Word) error
	AddPlayerWord(playerName, word, lobbyCode string) error
	GetPlayerWords(playerName, lobbyCode string) ([]string, error)
	DeletePlayerWordsByLobbyCode(lobbyCode string) error
	DeletePlayerWordsByPlayerAndLobbyCode(playerName, lobbyCode string) error
	SeedPlayerWords(lobbyCode string, game *Game) error
	GetWordCountByLobbyCode(lobbyCode string) ([]*PlayerWordCount, error)
	UpdateAccountWinsAndLosses(lobbyCode, winner string) error
	SetPlayerTargetWord(playerName, targetWord, lobbyCode string) error
	GetPlayerTargetWord(playerName, lobbyCode string) (string, error)
	IsPlayerWord(playerName, word, lobbyCode string) (bool, error)
	IncrementPlayerPoints(playerName, lobbyCode string, points int) error
	SetIsOwner(username string, setOwner bool) error
	SelectWinnerByPoints(lobbyCode string) (string, error)
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