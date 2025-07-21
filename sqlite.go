package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore() (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", "./store.db")
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// Release lock in 5 seconds
	// Reference: https://stackoverflow.com/questions/66909180/increase-the-lock-timeout-with-sqlite-and-what-is-the-default-values
	_, err = db.Exec("PRAGMA busy_timeout = 5000")
	if err != nil {
		return nil, err
	}

	fmt.Println("Connected to the SQLite database successfully.")
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) createAccountTable() error {
	query := `create table if not exists account (
		username text primary key,
		image_name text,
		password text,
		wins integer,
		losses integer,
		created_at datetime,
		status text
		)`
	_, err := s.db.Exec(query)
	return err
}

func (s *SQLiteStore) createImageTable() error {
	query := `create table if not exists image (
		name text primary key,
		data blob
		)`
	_, err := s.db.Exec(query)
	return err
}

func (s *SQLiteStore) createPlayerTable() error {
	query := `create table if not exists player (
		name text,
		lobby_code text,
		image_name,
		is_owner boolean,
		has_account boolean,
		primary key (name, lobby_code)
		)`
	_, err := s.db.Exec(query)
	return err
}

func (s *SQLiteStore) createLobbyTable() error {
	query := `create table if not exists lobby (
		name text,
		image_name text,
		lobby_code text,
		game_mode text,
		player_count integer,
		primary key (lobby_code)
		)`
	_, err := s.db.Exec(query)
	return err
}

func (s *SQLiteStore) Init() error {
	if err := s.createAccountTable(); err != nil {
		return err
	}
	if err := s.createImageTable(); err != nil {
		return err
	}
	if err := s.createPlayerTable(); err != nil {
		fmt.Println(err)
		return err
	}
	if err := s.createLobbyTable(); err != nil {
		return err
	}
	return nil

}

func (s *SQLiteStore) CreateAccount(acc *Account) error {
	query := `insert into account 
	(username, image_name, password, wins, losses, created_at, status)
	values (?, ?, ?, ?, ?, ?,?)`
	_, err := s.db.Exec(
		query,
		acc.Username,
		acc.ImageName,
		acc.Password,
		acc.Wins,
		acc.Losses,
		acc.CreatedAt,
		acc.Status,
	)
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) CreatePlayer(player *Player) error {
	query := `insert into player 
	(name, lobby_code, image_name, is_owner, has_account)
	values (?, ?, ?, ?, ?)`
	_, err := s.db.Exec(
		query,
		player.Name,
		player.LobbyCode,
		player.ImageName,
		player.IsOwner,
		player.HasAccount)
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) CreateLobby(lobby *Lobby) error {
	query := `insert into lobby 
	(name, image_name, lobby_code, game_mode, player_count)
	values (?, ?, ?, ?, ?)`
	_, err := s.db.Exec(
		query,
		lobby.Name,
		lobby.ImageName,
		lobby.LobbyCode,
		lobby.GameMode,
		lobby.PlayerCount,
	)
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) GetPlayerByLobbyCodeAndName(name, lobbyCode string) (*Player, error) {
	rows, err := s.db.Query("select * from player where name = ? and lobby_code = ?", name, lobbyCode)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		defer rows.Close()
		return scanIntoPlayer(rows)
	}
	return nil, fmt.Errorf("player %s not found", name)
}

func (s *SQLiteStore) DeletePlayer(name, lobbyCode string) error {
	_, err := s.db.Exec("delete from player where name = ? and lobby_code = ?", name, lobbyCode)
	return err
}

func (s *SQLiteStore) GetPlayersByLobbyCode(lobbyCode string) ([]*Player, error) {
	rows, err := s.db.Query("select * from player where lobby_code = ?", lobbyCode)
	if err != nil {
		return nil, err
	}
	players := []*Player{}
	for rows.Next() {
		player, err := scanIntoPlayer(rows)
		if err != nil {
			return nil, err
		}
		players = append(players, player)
	}
	return players, nil
}

func (s *SQLiteStore) GetAccountByUsername(username string) (*Account, error) {
	rows, err := s.db.Query("select * from account where username = ?", username)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		defer rows.Close()
		return scanIntoAccount(rows)
	}
	return nil, fmt.Errorf("account %s not found", username)
}

func (s *SQLiteStore) UpdateAccount(acc *Account) error {
	query := `update account set
	username = ?,
	image_name = ?,
	password = ?,
	wins = ?,
	losses = ?,
	status = ?
	where username = ?`
	_, err := s.db.Exec(
		query,
		acc.Username,
		acc.ImageName,
		acc.Password,
		acc.Wins,
		acc.Losses,
		acc.Status,
		acc.Username,
	)
	return err
}

func (s *SQLiteStore) AddImage(data []byte, name string) error {
	_, err := s.db.Exec(
		"insert or replace into image (name, data) values (?, ?)",
		name,
		data,
	)
	return err
}

func (s *SQLiteStore) GetImage(name string) ([]byte, error) {
	rows, err := s.db.Query("select data from image where name = ?", name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var image []byte
		err := rows.Scan(&image)
		if err != nil {
			return nil, err
		}
		return image, nil
	}
	return nil, fmt.Errorf("image for account %s not found", name)
}

func (s *SQLiteStore) GetImages() ([]*Image, error) {
	rows, err := s.db.Query("select * from image")
	if err != nil {
		return nil, err
	}
	images := []*Image{}
	for rows.Next() {
		img, err := scanIntoImage(rows)
		if err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, nil
}

func (s *SQLiteStore) NewImageForUsername(username string) string {
	images, err := s.GetImages()
	if err != nil {
		return err.Error()
	}
	size := len(images)
	//TODO: Use Radix
	hash := int(username[0]) % size
	image := images[hash]
	return image.Name
}

func (s *SQLiteStore) GetPlayerForAccount(username string) (*Player, error) {
	acc, err := s.GetAccountByUsername(username)
	if err != nil {
		return nil, err
	}
	return NewPlayer(username, "", acc.ImageName, false, true), nil
}

func (s *SQLiteStore) GetOwners() ([]*Player, error) {
	rows, err := s.db.Query("select * from player where is_owner = ?", true)
	if err != nil {
		return nil, err
	}
	owners := []*Player{}
	for rows.Next() {
		owner, err := scanIntoPlayer(rows)
		if err != nil {
			return nil, err
		}
		owners = append(owners, owner)
	}
	return owners, nil
}

func (s *SQLiteStore) GetLobbyForOwner(owner string) (string, error) {
	rows, err := s.db.Query("select lobby_code from player where name = ? and is_owner = ?", owner, true)
	if err != nil {
		return "", err
	}
	var lobbyCode string
	for rows.Next() {
		defer rows.Close()
		err := rows.Scan(&lobbyCode)
		if err != nil {
			return "", err
		}
		return lobbyCode, nil
	}
	return "", nil
}

func (s *SQLiteStore) DeletePlayersForLobby(lobbyCode string) error {
	_, err := s.db.Exec("delete from player where lobby_code = ?", lobbyCode)
	err = s.IncrementPlayerCount(lobbyCode, -1)
	return err
}

func (s *SQLiteStore) AddPlayerToLobby(lobbyCode string, player *Player) error {
	_, err := s.db.Exec(
		"insert into player (name, lobby_code, image_name, is_owner, has_account) values (?, ?, ?, ?, ?)",
		player.Name,
		lobbyCode,
		player.ImageName,
		player.IsOwner,
		player.HasAccount,
	)
	err = s.IncrementPlayerCount(lobbyCode, 1)
	return err
}

func (s * SQLiteStore) IncrementPlayerCount(lobbyCode string, increment int) error {
	_, err := s.db.Exec("update lobby set player_count = player_count + ? where lobby_code = ?", lobbyCode, increment)
	return err
}

func (s *SQLiteStore) GetLobbies() ([]*Lobby, error) {
	rows, err := s.db.Query("select * from lobby")
	if err != nil {
		return nil, err
	}
	lobbies := []*Lobby{}
	for rows.Next() {
		lobby, err := scanIntoLobby(rows)
		if err != nil {
			return nil, err
		}
		lobbies = append(lobbies, lobby)
	}
	return lobbies, nil
}

func (s *SQLiteStore) DeleteLobby(lobbyCode string) error {
	_, err := s.db.Exec("delete from lobby where lobby_code = ?", lobbyCode)
	return err
}


func (s *SQLiteStore) GetLobbyByCode(lobbyCode string) (*Lobby, error) {
	rows, err := s.db.Query("select * from lobby where lobby_code = ?", lobbyCode)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		defer rows.Close()
		return scanIntoLobby(rows)
	}
	return nil, fmt.Errorf("lobby %s not found", lobbyCode)
}

func (s *SQLiteStore) EditGameMode(lobbyCode, gameMode string) error {
	_, err := s.db.Exec("update lobby set game_mode = ? where lobby_code = ?", gameMode, lobbyCode)
	return err
}