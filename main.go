package main

// @title WomboCombo Go API
// @version 1.0
// @description This is the API for Wombo Combo Go
// @host localhost:3030
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

import (
	"flag"
	"log"
	"os"
	"github.com/joho/godotenv"
	_ "github.com/na50r/wombo-combo-go-be/docs"
)

var JWT_SECRET string
var CLIENT string
var ICONS string
var COMBINATIONS string
var WORDS string
var COHERE_API_KEY string
var POSTGRES_CONNECTION string

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
	COHERE_API_KEY = os.Getenv("COHERE_API_KEY")
	POSTGRES_CONNECTION = os.Getenv("POSTGRES_CONNECTION")

	if JWT_SECRET == "" {
		log.Fatal("JWT_SECRET not set")
	}
	if CLIENT == "" {
		log.Fatal("CLIENT not set")
	}
	if ICONS == "" {
		log.Fatal("ICONS not set")
	}
	if COMBINATIONS == "" {
		log.Fatal("COMBINATIONS not set")
	}
	if WORDS == "" {
		log.Fatal("WORDS not set")
	}
	if COHERE_API_KEY == "" {
		log.Fatal("COHERE_API_KEY not set")
	}
	if POSTGRES_CONNECTION == "" {
		log.Fatal("POSTGRES_CONNECTION not set")
	}
}


func main() {
	seed := flag.Bool("seed", false, "seed images & elements")
	flag.Parse()

	store, err := NewPostgresStore()
	if err != nil {
		log.Fatal(err)
	}

	if err := store.Init(); err != nil {
		log.Println("Error initializing database")
		log.Fatal(err)
	}

	//./bin/wc --seed
	if *seed {
		seedDatabase(store)
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
