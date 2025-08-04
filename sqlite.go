package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"math/rand"
	"strings"
	"time"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(name string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("./%s.db", name))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	_, err = db.Exec("PRAGMA journal_mode = WAL")
	if err != nil {
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
		status text,
		is_owner boolean
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
		target_word text,
		points integer,
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
func (s *SQLiteStore) createCombinationTable() error {
	query := `create table if not exists combination (
		a text,
		b text,
		result text,
		depth integer,
		primary key (a, b)
		)`
	_, err := s.db.Exec(query)
	return err
}

func (s *SQLiteStore) createWordTable() error {
	query := `create table if not exists word (
		word text primary key,
		depth integer,
		reachability float
		)`
	_, err := s.db.Exec(query)
	return err
}

func (s *SQLiteStore) createPlayerWordTable() error {
	query := `create table if not exists player_word (
		player_name text,
		word text,
		lobby_code text,
		timestamp datetime default current_timestamp,
		primary key (player_name, word, lobby_code)
		)`
	_, err := s.db.Exec(query)
	return err
}

func (s *SQLiteStore) createDailyWordTable() error {
	query := `create table if not exists daily_word (
		timestamp datetime default current_timestamp,
		word text
		)`
	_, err := s.db.Exec(query)
	return err
}

func (s *SQLiteStore) createDailyChallengeTable() error {
	query := `create table if not exists daily_challenge (
		timestamp datetime default current_timestamp,
		word_count integer,
		username text
		)`
	_, err := s.db.Exec(query)
	return err
}

func (s *SQLiteStore) createSessionTable() error {
	query := `create table if not exists session (
		id text primary key,
		refresh_token text,
		username text,
		is_revoked boolean,
		created_at datetime,
		expires_at datetime
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
	if err := s.createSessionTable(); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) AddDailyChallengeEntry(wordCount int, username string) error {
	today := time.Now().Format("2006-01-02")
	var oldCount int
	err := s.db.QueryRow("select word_count from daily_challenge where username = ? and timestamp = ?", username, today).Scan(&oldCount)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == sql.ErrNoRows {
		_, err = s.db.Exec("insert into daily_challenge (word_count, username, timestamp) values (?, ?, ?)", wordCount, username, today)
		return err
	}
	if oldCount > wordCount {
		_, err = s.db.Exec("update daily_challenge set word_count = ? where username = ? and timestamp = ?", wordCount, username, today)
		return err
	}
	return nil
}

func (s *SQLiteStore) CreateOrGetDailyWord(minReachability, maxReachability float64, maxDepth int) (string, error) {
	log.Println("Creating or getting daily word")
	today := time.Now().Format("2006-01-02")
	var word string
	err := s.db.QueryRow("select word from daily_word where timestamp = ?", today).Scan(&word)
	if err == sql.ErrNoRows {
		word, err := s.GetTargetWord(minReachability, maxReachability, maxDepth)
		if err != nil {
			return "", err
		}
		_, err = s.db.Exec("insert into daily_word (timestamp, word) values (?, ?)", today, word)
		if err != nil {
			return "", err
		}
		return word, nil
	}
	return word, nil
}

func (s *SQLiteStore) CreateAccount(acc *Account) error {
	query := `insert into account 
	(username, image_name, password, wins, losses, created_at, status, is_owner)
	values (?, ?, ?, ?, ?, ?, ?, ?)`
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

func (s *SQLiteStore) CreatePlayer(player *Player) error {
	query := `insert into player 
	(name, lobby_code, image_name, is_owner, has_account, target_word, points)
	values (?, ?, ?, ?, ?, ?, ?)`
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
	rows, err := s.db.Query("select * from image where name = ?", name)
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
	if len(images) > 1 {
		return nil, fmt.Errorf("Multiple images for name %s", name)
	}
	if len(images) == 0 {
		return nil, fmt.Errorf("Image for name %s not found", name)
	}
	return images[0].Data, nil
}

func (s *SQLiteStore) GetImages() ([]*Image, error) {
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

func (s *SQLiteStore) NewImageForUsername(username string) string {
	images, err := s.GetImages()
	if err != nil {
		return err.Error()
	}
	size := len(images)
	hash := RadixHash(username, size)
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

func (s *SQLiteStore) GetLobbyForOwner(owner string) (string, error) {
	rows, err := s.db.Query("select lobby_code from player where name = ? and is_owner = ?", owner, true)
	if err != nil {
		return "", err
	}
	lobbyCodes := []string{}
	var lobbyCode string
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&lobbyCode)
		if err != nil {
			return "", err
		}
		lobbyCodes = append(lobbyCodes, lobbyCode)
	}
	if len(lobbyCodes) > 1 {
		return "", fmt.Errorf("Multiple lobbies for owner")
	}
	if len(lobbyCodes) == 0 {
		log.Printf("No lobby found for owner %s", owner)
		return "", nil
	}
	return lobbyCodes[0], nil
}

func (s *SQLiteStore) DeletePlayersForLobby(lobbyCode string) error {
	_, err := s.db.Exec("delete from player where lobby_code = ?", lobbyCode)
	err = s.IncrementPlayerCount(lobbyCode, -1)
	return err
}

func (s *SQLiteStore) AddPlayerToLobby(lobbyCode string, player *Player) error {
	_, err := s.db.Exec(
		"insert into player (name, lobby_code, image_name, is_owner, has_account, target_word, points) values (?, ?, ?, ?, ?, ?, ?)",
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

func (s *SQLiteStore) IncrementPlayerCount(lobbyCode string, increment int) error {
	_, err := s.db.Exec("update lobby set player_count = player_count + ? where lobby_code = ?", increment, lobbyCode)
	return err
}

func (s *SQLiteStore) GetLobbies() ([]*Lobby, error) {
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

func (s *SQLiteStore) EditGameMode(lobbyCode string, gameMode GameMode) error {
	_, err := s.db.Exec("update lobby set game_mode = ? where lobby_code = ?", gameMode, lobbyCode)
	return err
}

func (s *SQLiteStore) GetCombination(a, b string) (*string, bool, error) {
	a, b = sortAB(a, b)
	var result string
	err := s.db.QueryRow("SELECT result FROM combination WHERE a = ? AND b = ?", a, b).Scan(&result)
	if err == sql.ErrNoRows {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}
	return &result, true, nil
}

func (s *SQLiteStore) AddCombination(combi *Combination) error {
	a, b := sortAB(combi.A, combi.B)
	_, err := s.db.Exec(
		"insert or ignore into combination (a, b, result, depth) values (?, ?, ?, ?)",
		a,
		b,
		combi.Result,
		combi.Depth,
	)
	return err
}

func (s *SQLiteStore) AddNewCombination(a, b, result string) error {
	a, b = sortAB(a, b)
	aDepth := 0
	bDepth := 0
	err := s.db.QueryRow("select depth from word where word = ?", a).Scan(&aDepth)
	if err != nil {
		return err
	}
	err = s.db.QueryRow("select depth from word where word = ?", b).Scan(&bDepth)
	if err != nil {
		return err
	}
	depth := max(aDepth, bDepth) + 1
	_, err = s.db.Exec(
		"insert or ignore into combination (a, b, result, depth) values (?, ?, ?, ?)",
		a,
		b,
		result,
		depth,
	)
	oldReachability := 0.0
	err = s.db.QueryRow("select reachability from word where word = ?", result).Scan(&oldReachability)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("No reachability for word %s", result)
	}
	reachability := 1.0 / float64(int(1)<<uint(depth))
	_, err = s.db.Exec(
		"insert or ignore into word (word, depth, reachability) values (?, ?, ?)",
		result,
		depth,
		reachability+oldReachability,
	)
	return err
}

func (s *SQLiteStore) AddWord(word *Word) error {
	w := strings.ToLower(word.Word)
	_, err := s.db.Exec(
		"insert or ignore into word (word, depth, reachability) values (?, ?, ?)",
		w,
		word.Depth,
		word.Reachability,
	)
	return err
}

func (s *SQLiteStore) GetTargetWords(minReachability, maxReachability float64, maxDepth int) ([]string, error) {
	rows, err := s.db.Query("select * from word where reachability >= ? and reachability <= ? and depth <= ?", minReachability, maxReachability, maxDepth)
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
	log.Printf("Number of target words: %d", len(targetWords))
	return targetWords, nil
}

func (s *SQLiteStore) GetTargetWord(minReachability, maxReachability float64, maxDepth int) (string, error) {
	targetWords, err := s.GetTargetWords(minReachability, maxReachability, maxDepth)
	if err != nil {
		return "", err
	}
	return targetWords[rand.Intn(len(targetWords))], nil
}

func (s *SQLiteStore) NewGame(lobbyCode string, gameMode GameMode, withTimer bool, duration int) (*Game, error) {
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

func (s *SQLiteStore) AddPlayerWord(playerName, word, lobbyCode string) error {
	_, err := s.db.Exec(
		"insert or ignore into player_word (player_name, word, lobby_code) values (?, ?, ?)",
		playerName,
		word,
		lobbyCode,
	)
	return err
}

func (s *SQLiteStore) IsPlayerWord(playerName, word, lobbyCode string) (bool, error) {
	var count int
	err := s.db.QueryRow("select count(*) from player_word where player_name = ? and word = ? and lobby_code = ?", playerName, word, lobbyCode).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *SQLiteStore) SetPlayerTargetWord(playerName, targetWord, lobbyCode string) error {
	_, err := s.db.Exec(
		"update player set target_word = ? where name = ? and lobby_code = ?",
		targetWord,
		playerName,
		lobbyCode,
	)
	return err
}

func (s *SQLiteStore) GetPlayerTargetWord(playerName, lobbyCode string) (string, error) {
	var targetWord string
	err := s.db.QueryRow("select target_word from player where name = ? and lobby_code = ?", playerName, lobbyCode).Scan(&targetWord)
	if err != nil {
		return "", err
	}
	return targetWord, nil
}

func (s *SQLiteStore) SeedPlayerWords(lobbyCode string, game *Game) error {
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

func (s *SQLiteStore) GetPlayerWords(playerName, lobbyCode string) ([]string, error) {
	rows, err := s.db.Query("select * from player_word where player_name = ? and lobby_code = ? order by timestamp asc", playerName, lobbyCode)
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

func (s *SQLiteStore) DeletePlayerWordsByLobbyCode(lobbyCode string) error {
	_, err := s.db.Exec("delete from player_word where lobby_code = ?", lobbyCode)
	return err
}

func (s *SQLiteStore) DeletePlayerWordsByPlayerAndLobbyCode(playerName, lobbyCode string) error {
	_, err := s.db.Exec("delete from player_word where player_name = ? and lobby_code = ?", playerName, lobbyCode)
	return err
}

func (s *SQLiteStore) GetWordCountByLobbyCode(lobbyCode string) ([]*PlayerWordCount, error) {
	query := `
	select player_name, COUNT(*) as word_count
	from player_word
	where lobby_code = ?
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

func (s *SQLiteStore) GetPlayersWithAccount(lobbyCode string) ([]*Account, error) {
	rows, err := s.db.Query("select * from player where lobby_code = ? and has_account = ?", lobbyCode, true)
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

func (s *SQLiteStore) UpdateAccountWinsAndLosses(lobbyCode, winner string) error {
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

func (s *SQLiteStore) IncrementPlayerPoints(playerName, lobbyCode string, points int) error {
	_, err := s.db.Exec("update player set points = points + ? where name = ? and lobby_code = ?", points, playerName, lobbyCode)
	return err
}

func (s *SQLiteStore) ResetPlayerPoints(lobbyCode string) error {
	_, err := s.db.Exec("update player set points = 0 where lobby_code = ?", lobbyCode)
	return err
}

func (s *SQLiteStore) SetIsOwner(username string, setOwner bool) error {
	if !setOwner {
		_, err := s.db.Exec("update account set is_owner = ? where username = ?", setOwner, username)
		return err
	}

	var isOwner bool
	err := s.db.QueryRow("select is_owner from account where username = ?", username).Scan(&isOwner)
	if err != nil {
		return err
	}
	if isOwner {
		return fmt.Errorf("user is already owner!")
	}
	_, err = s.db.Exec("update account set is_owner = ? where username = ?", setOwner, username)
	return err
}

func (s *SQLiteStore) SelectWinnerByPoints(lobbyCode string) (string, error) {
	rows, err := s.db.Query("select name from player where lobby_code = ? order by points desc", lobbyCode)
	if err != nil {
		return "", err
	}
	winners := []string{}
	var winner string
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&winner)
		if err != nil {
			return "", err
		}
		winners = append(winners, winner)
	}
	if len(winners) == 0 {
		return "", fmt.Errorf("No players found")
	}
	return winners[0], nil
}

func (s *SQLiteStore) DeleteAccount(username string) error {
	_, err := s.db.Exec("delete from account where username = ?", username)
	return err
}

func (s *SQLiteStore) GetChallengeEntries() ([]*ChallengeEntry, error) {
	today := time.Now().Format("2006-01-02")
	rows, err := s.db.Query("select * from daily_challenge where timestamp = ?", today)
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

func (s *SQLiteStore) GetImageByUsername(username string) ([]byte, error) {
	var imageName string
	err := s.db.QueryRow("select image_name from account where username = ?", username).Scan(&imageName)
	if err != nil {
		return nil, err
	}
	return s.GetImage(imageName)
}

func (s *SQLiteStore) CreateSession(session *Session) error {
	query := `insert into session 
	(id, refresh_token, username, is_revoked, created_at, expires_at)
	values (?, ?, ?, ?, ?, ?)`
	_, err := s.db.Exec(
		query,
		session.ID,
		session.RefreshToken,
		session.Username,
		session.IsRevoked,
		session.CreatedAt,
		session.ExpiresAt,
	)
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) GetSession(id string) (*Session, error) {
	rows, err := s.db.Query("select * from session where id = ?", id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		defer rows.Close()
		return scanIntoSession(rows)
	}
	return nil, fmt.Errorf("session %s not found", id)
}

func (s *SQLiteStore) RevokeSession(id string) error {
	_, err := s.db.Exec("update session set is_revoked = ? where id = ?", true, id)
	return err
}
