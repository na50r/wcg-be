package main

// @title WomboCombo Go API
// @version 1.0
// @description This is the API for Wombo Combo Go
// @host localhost:3030
// @BasePath /
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

// Current: Authorization: <Token>
// TODO: Authorization: Bearer <Token>
// Because of Current, use securityDefinitions.apikey rather than securityDefinitions.bearer
// https://swagger.io/docs/specification/v3_0/authentication/api-keys/
import (
	"log"
	"os"
	"github.com/joho/godotenv"
	"path/filepath"
	"strings"
	"flag"
	"encoding/csv"
	"strconv"
	_ "github.com/na50r/wombo-combo-go-be/docs"
)

var JWT_SECRET string
var CLIENT string
var ICONS string
var COMBINATIONS string
var WORDS string

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, continuing...")
	}
	JWT_SECRET = os.Getenv("JWT_SECRET")
	CLIENT = os.Getenv("CLIENT")
	ICONS = os.Getenv("ICONS")
	COMBINATIONS = os.Getenv("COMBINATIONS")
	WORDS = os.Getenv("WORDS")
}

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
	if err != nil {
		return err
	}
	for _, record := range records {
		combi := new(Combination)
		combi.A = strings.ToLower(record[3])
		combi.B = strings.ToLower(record[4])
		combi.Result = strings.ToLower(record[2])
		combi.Depth, _ = strconv.Atoi(record[1])
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

func main() {
	seed := flag.Bool("seed", false, "seed images & elements")
	flag.Parse()

	store, err := NewSQLiteStore()
	if err != nil {
		log.Fatal(err)
	}

	if err := store.Init(); err != nil {
		log.Fatal(err)
	}

	//./bin --seed
	if *seed {
		if err := setImages(store); err != nil {
			log.Fatal(err)
		}
		if err := setCombinations(store); err != nil {
			log.Fatal(err)
		}
		if err := setWords(store); err != nil {
			log.Fatal(err)
		}
	}

	//Accounts for ports provided by hosting services
	PORT := os.Getenv("PORT")
	if PORT == "" {
		PORT = "3030"
	}

	server := NewAPIServer(":"+PORT, store)
	log.Printf("Starting server on port %s", PORT)
	server.Run()
}
