package utility

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"github.com/gorilla/mux"
	dto "github.com/na50r/wombo-combo-go-be/dto"
)

func GetImageFromFilePath(filePath string) (string, []byte, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get absolute path: %w", err)
	}
	name := filepath.Base(absPath)

	imageExtRegex := regexp.MustCompile(`(?i)\.(png|jpe?g)$`)
	if !imageExtRegex.MatchString(name) {
		return "", nil, fmt.Errorf("unsupported image format: %s", name)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", nil, fmt.Errorf("failed to stat file: %w", err)
	}

	img := make([]byte, info.Size())
	_, err = f.Read(img)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read file: %w", err)
	}

	return name, img, nil
}

func GetFilePathsInDir(dir string) ([]string, error) {
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

func ReadImages(dirPath string) (map[string][]byte, error) {
	paths, err := GetFilePathsInDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get files in directory: %w", err)
	}
	images := make(map[string][]byte)
	for _, filePath := range paths {
		name, imageData, err := GetImageFromFilePath(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read image %s: %w", filePath, err)
		}
		if imageData == nil {
			log.Printf("Skipping image %s", name)
			continue 
		}
		images[name] = imageData
	}
	return images, nil
}


func ReadCSV(filePath string) ([][]string, error) {
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

func GetChannelID(r *http.Request) (int, error) {
	channelID := mux.Vars(r)["channelID"]
	return strconv.Atoi(channelID)
}

func GetUsername(r *http.Request) (string, error) {
	username := mux.Vars(r)["username"]
	return username, nil
}

func GetLobbyCode(r *http.Request) (string, error) {
	lobbyCode := mux.Vars(r)["lobbyCode"]
	return lobbyCode, nil
}

func GetPlayername(r *http.Request) (string, error) {
	playerName := mux.Vars(r)["playerName"]
	return playerName, nil
}

func PasswordValid(password string) error {
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

func CallCohereAPI(a, b, apiKey string) (string, error) {
	log.Println("Calling Cohere API")
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
	var apiResponse dto.CohereResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return "", err
	}
	newWord := FormatWord(apiResponse.Message.Content[0].Text)
	return newWord, nil
}

func FormatWord(word string) string {
	word = strings.ToLower(word)
	// Match all non-alphabetic characters
	re := regexp.MustCompile(`[^a-z]+`)
	word = re.ReplaceAllString(word, "")
	return word
}

func SortAB(a, b string) (string, string) {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	sorted := a < b
	if !sorted {
		a, b = b, a
	}
	return a, b
}

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}