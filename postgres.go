package main

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"time"
	"log"
	"math/rand"
	"strings"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore() (*PostgresStore, error) {
	//To avoid conflicts with dockerized postgres, make sure to create it with:
	//Local
	//docker run --name wc-postgres -e POSTGRES_PASSWORD=wc-local -p 5433:5432 -d postgres
	//Map port 5432 of the container to 5433 of the host
	//Adjust accordingly for deployment
	db, err := sql.Open("postgres", POSTGRES_CONNECTION)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	fmt.Println("Connected to the Postgres database successfully.")
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) createAccountTable() error {
	query := `create table if not exists account (
		username varchar(100) primary key,
		image_name varchar(100),
		password varchar(100),
		wins integer,
		losses integer,
		created_at timestamp,
		status text,
		is_owner boolean
		)`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStore) createImageTable() error {
	query := `create table if not exists image (
		name varchar(100) primary key,
		data bytea
		)`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStore) createPlayerTable() error {
	query := `create table if not exists player (
		name varchar(100),
		lobby_code varchar(100),
		image_name varchar(100),
		is_owner boolean,
		has_account boolean,
		target_word varchar(100),
		points integer,
		primary key (name, lobby_code)
		)`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStore) createLobbyTable() error {
	query := `create table if not exists lobby (
		name varchar(100),
		image_name varchar(100),
		lobby_code varchar(100),
		game_mode text,
		player_count integer,
		primary key (lobby_code)
		)`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStore) createCombinationTable() error {
	query := `create table if not exists combination (
		a varchar(100),
		b varchar(100),
		result varchar(100),
		depth integer,
		primary key (a, b)
		)`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStore) createWordTable() error {
	query := `create table if not exists word (
		word varchar(100) primary key,
		depth integer,
		reachability float
		)`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStore) createPlayerWordTable() error {
	query := `create table if not exists player_word (
		player_name varchar(100),
		word varchar(100),
		lobby_code varchar(100),
		timestamp timestamp default current_timestamp,
		primary key (player_name, word, lobby_code)
		)`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStore) createDailyWordTable() error {
	query := `create table if not exists daily_word (
		timestamp timestamp default current_timestamp,
		word varchar(100)
		)`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStore) createDailyChallengeTable() error {
	query := `create table if not exists daily_challenge (
		timestamp timestamp default current_timestamp,
		word_count integer,
		username varchar(100)
		)`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStore) Init() error {
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
	if err := s.createCombinationTable(); err != nil {
		return err
	}
	if err := s.createWordTable(); err != nil {
		return err
	}
	if err := s.createPlayerWordTable(); err != nil {
		return err
	}
	if err := s.createDailyWordTable(); err != nil {
		return err
	}
	if err := s.createDailyChallengeTable(); err != nil {
		return err
	}
	return nil
}

func (s *PostgresStore) AddDailyChallengeEntry(wordCount int, username string) error {
	today := time.Now().Format("2006-01-02")
	var oldCount int 
	err := s.db.QueryRow("select word_count from daily_challenge where username = $1 and timestamp = $2", username, today).Scan(&oldCount)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == sql.ErrNoRows {
		_, err = s.db.Exec("insert into daily_challenge (word_count, username, timestamp) values ($1, $2, $3)", wordCount, username, today)
		return err
	}
	if oldCount > wordCount {
		_, err = s.db.Exec("update daily_challenge set word_count = $1 where username = $2 and timestamp = $3", wordCount, username, today)
		return err
	}
	return nil
}


func (s *PostgresStore) CreateOrGetDailyWord(minReachability, maxReachability float64, maxDepth int) (string, error) {
	log.Println("Creating or getting daily word")
	today := time.Now().Format("2006-01-02")
	var word string
	err := s.db.QueryRow("select word from daily_word where timestamp = $1", today).Scan(&word)
	if err == sql.ErrNoRows {
		word, err := s.GetTargetWord(minReachability, maxReachability, maxDepth)
		if err != nil {
			return "", err
		}
		_, err = s.db.Exec("insert into daily_word (timestamp, word) values ($1, $2)", today, word)
		if err != nil {
			return "", err
		}
		return word, nil
	}
	return word, nil
}

func (s *PostgresStore) CreateAccount(acc *Account) error {
	query := `insert into account 
	(username, image_name, password, wins, losses, created_at, status, is_owner)
	values ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := s.db.Exec(
		query,
		acc.Username,
		acc.ImageName,
		acc.Password,
		acc.Wins,
		acc.Losses,
		acc.CreatedAt,
		acc.Status,
		acc.IsOwner,
	)
	if err != nil {
		return err
	}
	return nil
}

func (s *PostgresStore) CreatePlayer(player *Player) error {
	query := `insert into player 
	(name, lobby_code, image_name, is_owner, has_account, target_word, points)
	values ($1, $2, $3, $4, $5, $6, $7)`
	_, err := s.db.Exec(
		query,
		player.Name,
		player.LobbyCode,
		player.ImageName,
		player.IsOwner,
		player.HasAccount,
		player.TargetWord,
		player.Points,
	)
	if err != nil {
		return err
	}
	return nil
}

func (s *PostgresStore) CreateLobby(lobby *Lobby) error {
	query := `insert into lobby 
	(name, image_name, lobby_code, game_mode, player_count)
	values ($1, $2, $3, $4, $5)`
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

func (s *PostgresStore) GetPlayerByLobbyCodeAndName(name, lobbyCode string) (*Player, error) {
	rows, err := s.db.Query("select * from player where name = $1 and lobby_code = $2", name, lobbyCode)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		defer rows.Close()
		return scanIntoPlayer(rows)
	}
	return nil, fmt.Errorf("player %s not found", name)
}

func (s *PostgresStore) DeletePlayer(name, lobbyCode string) error {
	_, err := s.db.Exec("delete from player where name = $1 and lobby_code = $2", name, lobbyCode)
	return err
}

func (s *PostgresStore) GetPlayersByLobbyCode(lobbyCode string) ([]*Player, error) {
	rows, err := s.db.Query("select * from player where lobby_code = $1", lobbyCode)
	if err != nil {
		return nil, err
	}
	players := []*Player{}
	defer rows.Close()
	for rows.Next() {
		player, err := scanIntoPlayer(rows)
		if err != nil {
			return nil, err
		}
		players = append(players, player)
	}
	return players, nil
}

func (s *PostgresStore) GetAccountByUsername(username string) (*Account, error) {
	rows, err := s.db.Query("select * from account where username = $1", username)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		defer rows.Close()
		return scanIntoAccount(rows)
	}
	return nil, fmt.Errorf("account %s not found", username)
}

func (s *PostgresStore) UpdateAccount(acc *Account) error {
	query := `update account set
	username = $1,
	image_name = $2,
	password = $3,
	wins = 	$4,
	losses = $5,
	status = $6
	where username = $7`
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

func (s *PostgresStore) AddImage(data []byte, name string) error {
	_, err := s.db.Exec(
		"insert into image (name, data) values ($1, $2) on conflict (name) do update set data = $2",
		name,
		data,
	)
	return err
}

func (s *PostgresStore) GetImage(name string) ([]byte, error) {
	rows, err := s.db.Query("select data from image where name = $1", name)
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

func (s *PostgresStore) GetImages() ([]*Image, error) {
	rows, err := s.db.Query("select * from image")
	if err != nil {
		return nil, err
	}
	images := []*Image{}
	defer rows.Close()
	for rows.Next() {
		img, err := scanIntoImage(rows)
		if err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, nil
}

func (s *PostgresStore) NewImageForUsername(username string) string {
	images, err := s.GetImages()
	if err != nil {
		return err.Error()
	}
	size := len(images)
	hash := RadixHash(username, size)
	image := images[hash]
	return image.Name
}

func (s *PostgresStore) GetPlayerForAccount(username string) (*Player, error) {
	acc, err := s.GetAccountByUsername(username)
	if err != nil {
		return nil, err
	}
	return NewPlayer(username, "", acc.ImageName, false, true), nil
}

func (s *PostgresStore) GetOwners() ([]*Player, error) {
	rows, err := s.db.Query("select * from player where is_owner = $1", true)
	if err != nil {
		return nil, err
	}
	owners := []*Player{}
	defer rows.Close()
	for rows.Next() {
		owner, err := scanIntoPlayer(rows)
		if err != nil {
			return nil, err
		}
		owners = append(owners, owner)
	}
	return owners, nil
}

func (s *PostgresStore) GetLobbyForOwner(owner string) (string, error) {
	rows, err := s.db.Query("select lobby_code from player where name = $1 and is_owner = $2", owner, true)
	if err != nil {
		return "", err
	}
	var lobbyCode string
	defer rows.Close()
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

func (s *PostgresStore) DeletePlayersForLobby(lobbyCode string) error {
	_, err := s.db.Exec("delete from player where lobby_code = $1", lobbyCode)
	err = s.IncrementPlayerCount(lobbyCode, -1)
	return err
}

func (s *PostgresStore) AddPlayerToLobby(lobbyCode string, player *Player) error {
	_, err := s.db.Exec(
		"insert into player (name, lobby_code, image_name, is_owner, has_account, target_word, points) values ($1, $2, $3, $4, $5, $6, $7)",
		player.Name,
		lobbyCode,
		player.ImageName,
		player.IsOwner,
		player.HasAccount,
		player.TargetWord,
		player.Points,
	)
	log.Printf("insert error: %v", err)
	if err != nil {
		return err
	}
	err = s.IncrementPlayerCount(lobbyCode, 1)
	return err
}

func (s *PostgresStore) IncrementPlayerCount(lobbyCode string, increment int) error {
	_, err := s.db.Exec("update lobby set player_count = player_count + $1 where lobby_code = $2", increment, lobbyCode)
	return err
}

func (s *PostgresStore) GetLobbies() ([]*Lobby, error) {
	rows, err := s.db.Query("select * from lobby")
	if err != nil {
		return nil, err
	}
	lobbies := []*Lobby{}
	defer rows.Close()
	for rows.Next() {
		lobby, err := scanIntoLobby(rows)
		if err != nil {
			return nil, err
		}
		lobbies = append(lobbies, lobby)
	}
	return lobbies, nil
}

func (s *PostgresStore) DeleteLobby(lobbyCode string) error {
	_, err := s.db.Exec("delete from lobby where lobby_code = $1", lobbyCode)
	return err
}

func (s *PostgresStore) GetLobbyByCode(lobbyCode string) (*Lobby, error) {
	rows, err := s.db.Query("select * from lobby where lobby_code = $1", lobbyCode)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		defer rows.Close()
		return scanIntoLobby(rows)
	}
	return nil, fmt.Errorf("lobby %s not found", lobbyCode)
}

func (s *PostgresStore) EditGameMode(lobbyCode string, gameMode GameMode) error {
	_, err := s.db.Exec("update lobby set game_mode = $1 where lobby_code = $2", gameMode, lobbyCode)
	return err
}

func (s *PostgresStore) GetCombination(a, b string) (*string, bool, error) {
	a, b = sortAB(a, b)
	var result string
	err := s.db.QueryRow("select result from combination where a = $1 AND b = $2", a, b).Scan(&result)
	if err == sql.ErrNoRows {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}
	return &result, true, nil
}

func (s *PostgresStore) AddCombination(combi *Combination) error {
	a, b := sortAB(combi.A, combi.B)
	_, err := s.db.Exec(
		"insert into combination (a, b, result, depth) values ($1, $2, $3, $4) on conflict do nothing",
		a,
		b,
		combi.Result,
		combi.Depth,
	)
	return err
}

func (s *PostgresStore) AddNewCombination(a, b, result string) error {
	a, b = sortAB(a, b)
	aDepth := 0
	bDepth := 0
	err := s.db.QueryRow("select depth from word where word = $1", a).Scan(&aDepth)
	if err != nil {
		return err
	}
	err = s.db.QueryRow("select depth from word where word = $1", b).Scan(&bDepth)
	if err != nil {
		return err
	}
	depth := max(aDepth, bDepth) + 1
	_, err = s.db.Exec(
		"insert into combination (a, b, result, depth) values ($1, $2, $3, $4) on conflict do nothing",
		a,
		b,
		result,
		depth,
	)
	reachability := 1.0 / float64(int(1) << uint(depth))
	_, err = s.db.Exec(
		"insert into word (word, depth, reachability) values ($1, $2, $3) on conflict do nothing",
		result,
		depth,
		reachability,
	)
	return err
}


func (s *PostgresStore) AddWord(word *Word) error {
	w := strings.ToLower(word.Word)
	_, err := s.db.Exec(
		"insert into word (word, depth, reachability) values ($1, $2, $3) on conflict do nothing",
		w,
		word.Depth,
		word.Reachability,
	)
	return err
}

func (s *PostgresStore) GetTargetWords(minReachability, maxReachability float64, maxDepth int) ([]string, error) {
	rows, err := s.db.Query("select * from word where reachability >= $1 and reachability <= $2 and depth <= $3", minReachability, maxReachability, maxDepth)
	if err != nil {
		return nil, err
	}
	targetWords := []string{}
	defer rows.Close()
	for rows.Next() {
		word, err := scanIntoWord(rows)
		if err != nil {
			return nil, err
		}
		targetWords = append(targetWords, word.Word)
	}
	return targetWords, nil
}

func (s *PostgresStore) GetTargetWord(minReachability, maxReachability float64, maxDepth int) (string, error) {
	targetWords, err := s.GetTargetWords(minReachability, maxReachability, maxDepth)
	if err != nil {
		return "", err
	}
	return targetWords[rand.Intn(len(targetWords))], nil
}

func (s *PostgresStore) NewGame(lobbyCode string, gameMode GameMode, withTimer bool, duration int) (*Game, error) {
	game := new(Game)
	game.LobbyCode = lobbyCode
	game.GameMode = gameMode
	game.WithTimer = withTimer

	if withTimer {
		game.Timer = NewTimer(duration)
	}

	err := fmt.Errorf("game mode %s not found", gameMode)
	if gameMode == VANILLA {
		return game, nil
	}
	if gameMode == FUSION_FRENZY {
		game.TargetWord, err = s.GetTargetWord(0.4, 10, 10)
		if err != nil {
			return nil, err
		}
		return game, nil
	}
	if gameMode == WOMBO_COMBO {
		game.TargetWords, err = s.GetTargetWords(0.4, 10, 10)
		if err != nil {
			return nil, err
		}
		return game, nil
	}
	if gameMode == DAILY_CHALLENGE {
		game.TargetWord, err = s.CreateOrGetDailyWord(0.4, 8, 8)
		if err != nil {
			log.Printf("Error creating or getting daily word: %v", err)
			return nil, err
		}
		return game, nil
	}
	return nil, err
}

func (s *PostgresStore) AddPlayerWord(playerName, word, lobbyCode string) error {
	_, err := s.db.Exec(
		"insert into player_word (player_name, word, lobby_code) values ($1, $2, $3) on conflict do nothing",
		playerName,
		word,
		lobbyCode,
	)
	return err
}

func (s *PostgresStore) IsPlayerWord(playerName, word, lobbyCode string) (bool, error) {
	var count int
	err := s.db.QueryRow("select count(*) from player_word where player_name = $1 and word = $2 and lobby_code = $3", playerName, word, lobbyCode).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *PostgresStore) SetPlayerTargetWord(playerName, targetWord, lobbyCode string) error {
	_, err := s.db.Exec(
		"update player set target_word = $1 where name = $2 and lobby_code = $3",
		targetWord,
		playerName,
		lobbyCode,
	)
	return err
}

func (s *PostgresStore) GetPlayerTargetWord(playerName, lobbyCode string) (string, error) {
	var targetWord string
	err := s.db.QueryRow("select target_word from player where name = $1 and lobby_code = $2", playerName, lobbyCode).Scan(&targetWord)
	if err != nil {
		return "", err
	}
	return targetWord, nil
}

func (s *PostgresStore) SeedPlayerWords(lobbyCode string, game *Game) error {
	players, err := s.GetPlayersByLobbyCode(lobbyCode)
	if err != nil {
		return err
	}
	for _, player := range players {
		target, err := game.SetTarget()
		if err != nil {
			return err
		}
		if err := s.SetPlayerTargetWord(player.Name, target, lobbyCode); err != nil {
			return err
		}
		s.AddPlayerWord(player.Name, "fire", lobbyCode)
		s.AddPlayerWord(player.Name, "water", lobbyCode)
		s.AddPlayerWord(player.Name, "earth", lobbyCode)
		s.AddPlayerWord(player.Name, "wind", lobbyCode)
	}
	return nil
}

func (s *PostgresStore) GetPlayerWords(playerName, lobbyCode string) ([]string, error) {
	rows, err := s.db.Query("select * from player_word where player_name = $1 and lobby_code = $2 order by timestamp asc", playerName, lobbyCode)
	if err != nil {
		return nil, err
	}
	words := []string{}
	defer rows.Close()
	for rows.Next() {
		playerWord, err := scanIntoPlayerWord(rows)
		if err != nil {
			return nil, err
		}
		words = append(words, playerWord.Word)
	}
	return words, nil
}

func (s *PostgresStore) DeletePlayerWordsByLobbyCode(lobbyCode string) error {
	_, err := s.db.Exec("delete from player_word where lobby_code = $1", lobbyCode)
	return err
}

func (s *PostgresStore) DeletePlayerWordsByPlayerAndLobbyCode(playerName, lobbyCode string) error {
	_, err := s.db.Exec("delete from player_word where player_name = $1 and lobby_code = $2", playerName, lobbyCode)
	return err
}

func (s *PostgresStore) GetWordCountByLobbyCode(lobbyCode string) ([]*PlayerWordCount, error) {
	query := `
	select player_name, COUNT(*) as word_count
	from player_word
	where lobby_code = $1
	group by player_name
	order by word_count desc
	`
	rows, err := s.db.Query(query, lobbyCode)
	if err != nil {
		return nil, err
	}
	wordCounts := []*PlayerWordCount{}
	defer rows.Close()
	for rows.Next() {
		wordCount, err := scanIntoPlayerWordCount(rows)
		if err != nil {
			return nil, err
		}
		wordCount.WordCount -= 4 // Exclude starting words
		wordCounts = append(wordCounts, wordCount)
	}
	return wordCounts, nil
}

func (s *PostgresStore) GetPlayersWithAccount(lobbyCode string) ([]*Account, error) {
	rows, err := s.db.Query("select * from player where lobby_code = $1 and has_account = $2", lobbyCode, true)
	if err != nil {
		return nil, err
	}
	accounts := []*Account{}
	defer rows.Close()
	for rows.Next() {
		player, err := scanIntoPlayer(rows)
		if err != nil {
			return nil, err
		}
		acc, err := s.GetAccountByUsername(player.Name)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, acc)
	}
	return accounts, nil
}

func (s *PostgresStore) UpdateAccountWinsAndLosses(lobbyCode, winner string) error {
	accounts, err := s.GetPlayersWithAccount(lobbyCode)
	if err != nil {
		return err
	}
	for _, acc := range accounts {
		if acc.Username == winner {
			acc.Wins++
		} else {
			acc.Losses++
		}
		if err := s.UpdateAccount(acc); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) IncrementPlayerPoints(playerName, lobbyCode string, points int) error {
	_, err := s.db.Exec("update player set points = points + $1 where name = $2 and lobby_code = $3", points, playerName, lobbyCode)
	return err
}

func (s *PostgresStore) ResetPlayerPoints(lobbyCode string) error { 
	_, err := s.db.Exec("update player set points = 0 where lobby_code = $1", lobbyCode)
	return err
}

func (s *PostgresStore) SetIsOwner(username string, setOwner bool) error {
	if !setOwner {
		_, err := s.db.Exec("update account set is_owner = $1 where username = $2", setOwner, username)
		return err
	}

	var isOwner bool
	err := s.db.QueryRow("select is_owner from account where username = $1", username).Scan(&isOwner)
	if err != nil {
		return err
	}
	if isOwner {
		return fmt.Errorf("user is already owner!")
	}
	_, err = s.db.Exec("update account set is_owner = $1 where username = $2", setOwner, username)
	return err
}

func (s *PostgresStore) SelectWinnerByPoints(lobbyCode string) (string, error) {
	rows, err := s.db.Query("select name from player where lobby_code = $1 order by points desc", lobbyCode)
	if err != nil {
		return "", err
	}
	var winner string
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&winner)
		if err != nil {
			return "", err
		}
		return winner, nil
	}
	return "", nil
}

func (s *PostgresStore) DeleteAccount(username string) error {
	_, err := s.db.Exec("delete from account where username = $1", username)
	return err
}

func (s *PostgresStore) GetChallengeEntries() ([]*ChallengeEntry, error) {
	today := time.Now().Format("2006-01-02")
	rows, err := s.db.Query("select * from daily_challenge where timestamp = $1", today)
	if err != nil {
		return nil, err
	}
	entries := []*ChallengeEntry{}
	defer rows.Close()
	for rows.Next() {
		entry, err := scanIntoChallengeEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (s *PostgresStore) GetImageByUsername(username string) ([]byte, error) {
	var imageName string
	err := s.db.QueryRow("select image_name from account where username = $1", username).Scan(&imageName)
	if err != nil {
		return nil, err
	}
	return s.GetImage(imageName)
}
