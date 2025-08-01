package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
	"unicode"

	"github.com/gorilla/mux"

	"math/rand"

	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	jwt "github.com/golang-jwt/jwt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func getImageFromFilePath(filePath string) (*Image, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	absPath, _ := filepath.Abs(filePath)
	name := filepath.Base(absPath)
	if !strings.HasSuffix(name, ".png") {
		return nil, nil
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	img := make([]byte, info.Size())
	_, err = f.Read(img)
	if err != nil {
		return nil, err
	}
	image := new(Image)
	image.Data = img
	image.Name = name
	return image, nil
}

func getFilePathsInDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() {
			fullPath := filepath.Join(dir, entry.Name())
			paths = append(paths, fullPath)
		}
	}
	return paths, nil
}

func readImages() ([]*Image, error) {
	paths, err := getFilePathsInDir(ICONS)
	if err != nil {
		return nil, err
	}
	images := []*Image{}
	for _, path := range paths {
		image, err := getImageFromFilePath(path)
		if err != nil {
			return nil, err
		}
		if image == nil {
			continue
		}
		images = append(images, image)
	}
	return images, nil
}

func setImages(store Storage) error {
	images, err := readImages()
	if err != nil {
		return err
	}
	for _, image := range images {
		if err := store.AddImage(image.Data, image.Name); err != nil {
			return err
		}
	}
	return nil
}

func readCSV(filePath string) ([][]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	return records[1:], nil
}

func setCombinations(store Storage) error {
	records, err := readCSV(COMBINATIONS)
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

func setWords(store Storage) error {
	records, err := readCSV(WORDS)
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

func seedDatabase(store Storage) {
	log.Println("Seeding database...")
	if err := setImages(store); err != nil {
		log.Fatal(err)
	}
	log.Println("Images seeded")
	if err := setCombinations(store); err != nil {
		log.Fatal(err)
	}
	log.Println("Combinations seeded")
	if err := setWords(store); err != nil {
		log.Fatal(err)
	}
	log.Println("Words seeded")
}

func getChannelID(r *http.Request) (int, error) {
	channelID := mux.Vars(r)["channelID"]
	return strconv.Atoi(channelID)
}

func getUsername(r *http.Request) (string, error) {
	username := mux.Vars(r)["username"]
	return username, nil
}

func getLobbyCode(r *http.Request) (string, error) {
	lobbyCode := mux.Vars(r)["lobbyCode"]
	return lobbyCode, nil
}

func getPlayername(r *http.Request) (string, error) {
	playerName := mux.Vars(r)["playerName"]
	return playerName, nil
}

func createJWT(account *Account) (string, error) {
	claims := &jwt.MapClaims{
		"exp":      time.Now().Add(time.Hour * 12).Unix(),
		"username": account.Username,
		"type":     "account",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(JWT_SECRET))
}

func createLobbyToken(player *Player) (string, error) {
	claims := &jwt.MapClaims{
		"exp":        time.Now().Add(time.Hour * 4).Unix(),
		"playerName": player.Name,
		"lobbyCode":  player.LobbyCode,
		"hasAccount": player.HasAccount,
		"isOwner":    player.IsOwner,
		"type":       "player",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(JWT_SECRET))
}

func parseJWT(tokenString string) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(JWT_SECRET), nil
	})
}

func getToken(r *http.Request) (jwt.Token, bool, error) {
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		// Anon player
		return jwt.Token{}, false, nil
	}
	tokenString = strings.Replace(tokenString, "Bearer ", "", 1)
	token, err := parseJWT(tokenString)
	if err != nil && token != nil && !token.Valid {
		return jwt.Token{}, true, fmt.Errorf("unauthorized")
	}
	return *token, true, nil
}

func passwordValid(password string) error {
	if len(password) < 2 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if len(password) > 20 {
		return fmt.Errorf("password must be less than 20 characters")
	}
	if !IsLetter(password) {
		return fmt.Errorf("password must contain a letter")
	}
	return nil
}

func IsLetter(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func RadixHash(s string, size int) int {
	result := ""
	for _, ch := range s {
		result += strconv.Itoa(int(ch))
	}
	resultInt, _ := strconv.Atoi(result)
	hash := resultInt % size
	return hash
}

func (g *Game) SetTarget() (string, error) {
	if g.GameMode == VANILLA {
		return "", nil
	}
	if g.GameMode == WOMBO_COMBO {
		log.Println("Number of target words ", len(g.TargetWords))
		targetWord := g.TargetWords[rand.Intn(len(g.TargetWords))]
		return targetWord, nil
	}
	if g.GameMode == FUSION_FRENZY {
		return g.TargetWord, nil
	}
	if g.GameMode == DAILY_CHALLENGE {
		return g.TargetWord, nil
	}
	return "", fmt.Errorf("game mode %s not found", g.GameMode)
}

func ProcessMove(server *APIServer, game *Game, player *Player, result string) error {
	if game.GameMode == FUSION_FRENZY && player.TargetWord == result {
		game.StopTimer()
		game.Winner = player.Name
		if err := server.store.UpdateAccountWinsAndLosses(game.LobbyCode, player.Name); err != nil {
			return err
		}
		server.PublishToLobby(game.LobbyCode, Message{Data: GAME_OVER})
		server.PublishToLobby(game.LobbyCode, Message{Data: ACCOUNT_UPDATE})
		return nil
	}
	if game.GameMode == WOMBO_COMBO && player.TargetWord == result {
		var newTargetWord string
		var err error
		for {
			newTargetWord, err = game.SetTarget()
			if err != nil {
				return err
			}
			if newTargetWord != player.TargetWord {
				break
			}
		}
		log.Printf("Player %s reached target word %s, new target word is %s", player.Name, player.TargetWord, newTargetWord)
		if err := server.store.SetPlayerTargetWord(player.Name, newTargetWord, game.LobbyCode); err != nil {
			return err
		}
		if err := server.store.IncrementPlayerPoints(player.Name, game.LobbyCode, 10); err != nil {
			return err
		}
		server.PublishToLobby(game.LobbyCode, Message{Data: WOMBO_COMBO_EVENT})
	}
	if game.GameMode == DAILY_CHALLENGE && player.TargetWord == result {
		wordCounts, err := server.store.GetWordCountByLobbyCode(game.LobbyCode)
		if err != nil {
			return err
		}
		wordCount := wordCounts[0].WordCount
		log.Printf("Player %s completed daily challenge with word count %d", player.Name, wordCount)
		if err := server.store.AddDailyChallengeEntry(wordCount, player.Name); err != nil {
			return err
		}
		server.PublishToLobby(game.LobbyCode, Message{Data: GAME_OVER})
		return nil
	}
	if err := server.store.AddPlayerWord(player.Name, result, game.LobbyCode); err != nil {
		return err
	}
	return nil
}

func (g *Game) StartTimer(s *APIServer) {
	if g.WithTimer {
		g.Timer.Start(s, g.LobbyCode, g)
	}
}

func (g *Game) StopTimer() {
	if g.WithTimer {
		g.Timer.Stop()
	}
}

func (mt *Timer) Start(s *APIServer, lobbyCode string, game *Game) error {
	if mt.durationMinutes < 1 {
		return fmt.Errorf("duration must be at least 1 minute")
	}
	if mt.durationMinutes >= 5 {
		return fmt.Errorf("duration must be less than or equal to 5 minutes")
	}
	ctx, cancel := context.WithCancel(context.Background())
	mt.cancelFunc = cancel
	ticker := time.NewTicker(time.Second)
	total := mt.durationMinutes * 60
	half := total / 2
	quarter := half / 2
	three_quarter := half + quarter
	now := time.Now()
	triggers := map[int]bool{three_quarter: false, half: false, quarter: false}
	publishTimeEvent := func(secondsLeft int) {
		s.PublishToLobby(lobbyCode, Message{Data: TimeEvent{SecondsLeft: secondsLeft}})
	}
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				s.PublishToLobby(lobbyCode, Message{Data: TIMER_STOPPED})
				log.Printf("Timer %s stopped\n", lobbyCode)
				return
			case t := <-ticker.C:
				elapsed := int(t.Sub(now).Seconds())
				secondsLeft := total - elapsed
				log.Printf("Timer %s: %ds left\n", lobbyCode, secondsLeft)
				switch {
				case secondsLeft <= three_quarter && triggers[three_quarter] == false:
					triggers[three_quarter] = true
					publishTimeEvent(secondsLeft)
				case secondsLeft <= half && triggers[half] == false:
					triggers[half] = true
					publishTimeEvent(secondsLeft)
				case secondsLeft <= quarter && triggers[quarter] == false:
					triggers[quarter] = true
					publishTimeEvent(secondsLeft)
				case secondsLeft <= 10 && secondsLeft > 0:
					publishTimeEvent(secondsLeft)
				case secondsLeft <= 0:
					var err error
					game.Winner, err = s.store.SelectWinnerByPoints(lobbyCode)
					if err != nil {
						log.Printf("Error selecting winner: %v", err)
					}
					s.PublishToLobby(lobbyCode, Message{Data: GAME_OVER})
					return
				}
			}
		}
	}()
	return nil
}

func (mt *Timer) Stop() {
	mt.cancelFunc()
}

func GetCombination(store Storage, a, b string) (string, bool, error) {
	result, inDB, err := store.GetCombination(a, b)
	if err != nil {
		return "", false, err
	}
	if !inDB {
		newWord, err := CallCohereAPI(a, b)
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

func CallRandomWordAPI() (string, error) {
	log.Println("Calling random word API")
	endpoint := "https://random-word-api.herokuapp.com/word"
	resp, err := http.Get(endpoint)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var wordList []string
	if err := json.NewDecoder(resp.Body).Decode(&wordList); err != nil {
		return "", err
	}

	if len(wordList) == 0 {
		return "", fmt.Errorf("empty word list received")
	}
	return wordList[0], nil
}

func CallCohereAPI(a, b string) (string, error) {
	log.Println("Calling Cohere API")
	apiKey := COHERE_API_KEY
	url := "https://api.cohere.ai/v2/chat"

	body := map[string]interface{}{
		"model": "command-r",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": fmt.Sprintf("Given two words, come up with a new word that makes logical sense based on the two initial ones. Respond with nothing else but the new word. Example: Fire + Water = Steam\n\n Task: %s + %s = ?", a, b),
			},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	var apiResponse CohereResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return "", err
	}
	newWord := formatWord(apiResponse.Message.Content[0].Text)
	return newWord, nil
}

func formatWord(word string) string {
	word = strings.ToLower(word)
	// Match all non-alphabetic characters
	re := regexp.MustCompile(`[^a-z]+`)
	word = re.ReplaceAllString(word, "")
	return word
}

func sortAB(a, b string) (string, string) {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	sorted := a < b
	if !sorted {
		a, b = b, a
	}
	return a, b
}
