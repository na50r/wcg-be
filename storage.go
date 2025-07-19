package main

import (
	"database/sql"
)

type Storage interface {
	GetAccountByUsername(username string) (*Account, error)
	CreateAccount(a *Account) error
	UpdateAccount(a *Account) error
	AddImage(data []byte, name string) error
	GetImage(name string) ([]byte, error)
	GetImages() ([]*Image, error)
	NewImageForAccount(username string) string
	GetPlayerForAccount(username string) (*Player, error)
	CreatePlayer(player *Player) error
	GetPlayersByLobbyID(lobbyID string) ([]*Player, error)
	GetOwners() ([]*Player, error)
	GetLobbyForOwner(owner string) (string, error)
	DeleteLobby(lobbyID string) error
	Init() error
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
		&player.LobbyID,
		&player.ImageName,
		&player.IsOwner,
	)
	return player, err
}
