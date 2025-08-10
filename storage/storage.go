package storage

import (
	"database/sql"
	"time"
	c "github.com/na50r/wombo-combo-go-be/constants"
	"golang.org/x/crypto/bcrypt"
	"log"
	"strconv"
	"strings"
	dto "github.com/na50r/wombo-combo-go-be/dto"
	u "github.com/na50r/wombo-combo-go-be/utility"
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
	EditGameMode(lobbyCode string, gameMode c.GameMode) error
	AddCombination(element *Combination) error
	GetCombination(a, b string) (*string, bool, error)
	AddWord(word *Word) error
	AddPlayerWord(playerName, word, lobbyCode string) error
	GetPlayerWords(playerName, lobbyCode string) ([]string, error)
	DeletePlayerWordsByLobbyCode(lobbyCode string) error
	DeletePlayerWordsByPlayerAndLobbyCode(playerName, lobbyCode string) error
	GetWordCountByLobbyCode(lobbyCode string) ([]*dto.PlayerWordCount, error)
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
	GetAchievements() ([]*AchievementEntry, error)
	UpdateAccountWordCount(username string, newWordCount, wordCount int) error
	UpdatePlayerWordCount(playerName, lobbyCode string, newWordCount, wordCount int) error
	AddAchievement(entry *AchievementEntry) error
	UnlockAchievement(username, achievementTitle string) (bool, error)
	AddAchievementImage(data []byte, name string) error
	GetAchievementImage(name string) ([]byte, error)
	GetAchievementsForUser(username string) ([]string, error)
	GetAchievementByTitle(title string) (*AchievementEntry, error)
}


// DB Types
type Account struct {
	Username  string `db:"username"`
	Password  string `db:"password"`
	Wins      int    `db:"wins"`
	Losses    int    `db:"losses"`
	ImageName string `db:"image_name"`
	CreatedAt string `db:"created_at"`
	Status    c.Status `db:"status"`
	IsOwner   bool   `db:"is_owner"`
	NewWordCount int `db:"new_word_count"`
	WordCount int `db:"word_count"`
}

type Player struct {
	LobbyCode  string `db:"lobby_code"`
	Name       string `db:"name"`
	ImageName  string `db:"image_name"`
	IsOwner    bool   `db:"is_owner"`
	HasAccount bool   `db:"has_account"`
	TargetWord string `db:"target_word"`
	Points     int    `db:"points"`
	WordCount int `db:"word_count"`
	NewWordCount int `db:"new_word_count"`
}

type Lobby struct {
	Name        string   `db:"name"`
	ImageName   string   `db:"image_name"`
	LobbyCode   string   `db:"lobby_code"`
	GameMode    c.GameMode `db:"game_mode"`
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

type AchievementEntry struct {
	ID int `db:"id"`
	Title string `db:"title"`
	Type c.Achievement `db:"type"`
	Value string `db:"value"`
	Description string `db:"description"`
	ImageName string `db:"image_name"`
}

type Unlocked struct {
	Username string `db:"username"`
	AchievmentTitle string `db:"achievement_title"`
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
		Status:    c.OFFLINE,
		IsOwner:   false,
		NewWordCount: 0,
		WordCount: 0,
	}, nil
}

func NewPlayer(name, lobbyCode, imageName string, isOwner, hasAccount bool, newWordCount, wordCount int) *Player {
	return &Player{
		LobbyCode:  lobbyCode,
		Name:       name,
		ImageName:  imageName,
		IsOwner:    isOwner,
		HasAccount: hasAccount,
		TargetWord: "",
		Points:     0,
		WordCount: 0,
		NewWordCount: 0,
	}
}

func NewLobby(name, lobbyCode, imageName string) *Lobby {
	return &Lobby{
		Name:        name,
		ImageName:   imageName,
		LobbyCode:   lobbyCode,
		GameMode:    c.VANILLA,
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
		&acc.NewWordCount,
		&acc.WordCount,
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
		&player.WordCount,
		&player.NewWordCount,
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

func scanIntoPlayerWordCount(rows *sql.Rows) (*dto.PlayerWordCount, error) {
	wordCount := new(dto.PlayerWordCount)
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

func scanIntoAchievementEntry(rows *sql.Rows) (*AchievementEntry, error) {
	entry := new(AchievementEntry)
	err := rows.Scan(
		&entry.ID,
		&entry.Title,
		&entry.Type,
		&entry.Value,
		&entry.Description,
		&entry.ImageName,
	)
	return entry, err
}

func scanIntoUnlocked(rows *sql.Rows) (*Unlocked, error) {
	unlocked := new(Unlocked)
	err := rows.Scan(
		&unlocked.Username,
		&unlocked.AchievmentTitle,
	)
	return unlocked, err
}

func SetAchievementImages(store Storage, aIconPath string) error {
	images, err := u.ReadImages(aIconPath)
	if err != nil {
		return err
	}
	for name, image := range images {
		if err := store.AddAchievementImage(image, name); err != nil {
			return err
		}
	}
	return nil
}

func SetImages(store Storage, iconPath string) error {
	images, err := u.ReadImages(iconPath)
	if err != nil {
		return err
	}
	for name, image := range images {
		if err := store.AddImage(image, name); err != nil {
			return err
		}
	}
	return nil
}


func SetAchievements(store Storage, aPath string) error {
	records, err := u.ReadCSV(aPath)
	if err != nil {
		return err
	}
	log.Println("Number of achievements ", len(records))
	for _, record := range records {
		entry := new(AchievementEntry)
		entry.Title = record[0]
		entry.Type = c.Achievement(record[1])
		entry.Value = strings.ToLower(record[2])
		entry.Description = record[3]
		entry.ImageName = record[4]
		if err := store.AddAchievement(entry); err != nil {
			return err
		}
	}
	return nil
}

func SetCombinations(store Storage, combiPath string) error {
	records, err := u.ReadCSV(combiPath)
	log.Println("Number of combinations ", len(records))
	if err != nil {
		return err
	}
	for _, record := range records {
		combi := new(Combination)
		combi.A = strings.ToLower(record[1])
		combi.B = strings.ToLower(record[2])
		combi.Result = strings.ToLower(record[3])
		combi.Depth, _ = strconv.Atoi(record[0])
		if err := store.AddCombination(combi); err != nil {
			return err
		}
	}
	return nil
}

func SetWords(store Storage, wordPath string) error {
	records, err := u.ReadCSV(wordPath)
	if err != nil {
		return err
	}
	log.Println("Number of words ", len(records))
	for _, record := range records {
		word := new(Word)
		word.Word = strings.ToLower(record[0])
		word.Depth, _ = strconv.Atoi(record[1])
		word.Reachability, _ = strconv.ParseFloat(record[2], 64)
		if err := store.AddWord(word); err != nil {
			return err
		}
	}
	return nil
}

func SeedDB(store Storage, wordPath, combiPath, iconPath, aIconPath, aPath string) {
	log.Println("Seeding database...")
	if err := SetImages(store, iconPath); err != nil {
		log.Fatal(err)
	}
	log.Println("Images seeded")
	if err := SetCombinations(store, combiPath); err != nil {
		log.Fatal(err)
	}
	log.Println("Combinations seeded")
	if err := SetWords(store, wordPath); err != nil {
		log.Fatal(err)
	}
	log.Println("Words seeded")
	if err := SetAchievements(store, aPath); err != nil {
		log.Fatal(err)
	}
	log.Println("Achievements seeded")
	if err := SetAchievementImages(store, aIconPath); err != nil {
		log.Fatal(err)
	}
	log.Println("Achievement images seeded")
}


func GetCombination(store Storage, a, b, apiKey string) (string, bool, error) {
	result, inDB, err := store.GetCombination(a, b)
	if err != nil {
		return "", false, err
	}
	if !inDB {
		newWord, err := u.CallCohereAPI(a, b, apiKey)
		if err != nil {
			log.Printf("Error calling Cohere API: %v", err)
			return "star", false, nil
		}
		log.Printf("Adding new combination %s + %s = %s", a, b, newWord)
		err = store.AddNewCombination(a, b, newWord)
		if err != nil {
			log.Printf("Error adding new combination: %v", err)
			return "star", false, nil
		}
		return newWord, true, nil
	}
	return *result, false, nil
}
