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