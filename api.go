package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger"
)

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

// Allows error handling
type APIFunc func(http.ResponseWriter, *http.Request) error

func makeHTTPHandleFunc(f APIFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			WriteJSON(w, http.StatusBadRequest, APIError{Error: err.Error()})
		}
	}
}

type APIServer struct {
	router        *mux.Router
	listenAddr    string
	store         Storage
	broker        *Broker
	lobbyClients  map[string]map[int]bool // Maps a lobby code to a SET of clients
	playerClient  map[string]int          // Maps each player to a client
	games         map[string]*Game
	achievements  AchievementMaps
}

func NewAPIServer(listenAddr string, store Storage) *APIServer {
	s := APIServer{
		router:        mux.NewRouter(),
		listenAddr:    listenAddr,
		store:         store,
		broker:        NewBroker(),
		lobbyClients:  make(map[string]map[int]bool),
		playerClient:  make(map[string]int),
		games:         make(map[string]*Game),
	}
	go s.listen()
	s.SetupAchievements()
	return &s
}

func corsMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", CLIENT)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func (s *APIServer) RegisterRoutes() error {
	router := s.router
	router.Use(corsMiddleware)
	//Endpoints
	router.HandleFunc("/login", makeHTTPHandleFunc(s.handleLogin))
	router.HandleFunc("/logout", makeHTTPHandleFunc(s.handleLogout))

	router.HandleFunc("/accounts", makeHTTPHandleFunc(s.handleRegister))
	router.HandleFunc("/account/{username}", withAccountAuth(makeHTTPHandleFunc(s.handleAccount)))
	router.HandleFunc("/account/{username}/images", withAccountAuth(makeHTTPHandleFunc(s.handleGetImages)))
	router.HandleFunc("/account/{username}/leaderboard", withAccountAuth(makeHTTPHandleFunc(s.handleLeaderboard)))
	router.HandleFunc("/account/{username}/achievements", withAccountAuth(makeHTTPHandleFunc(s.handleAchievements)))

	// Lobby Endpoints
	router.HandleFunc("/lobbies", makeHTTPHandleFunc(s.handleLobbies))
	router.HandleFunc("/lobbies/{lobbyCode}/{playerName}", withPlayerAuth(makeHTTPHandleFunc(s.handleGetLobby)))
	router.HandleFunc("/lobbies/{lobbyCode}/{playerName}/leave", withPlayerAuth(makeHTTPHandleFunc(s.handleLeaveLobby)))
	router.HandleFunc("/lobbies/{lobbyCode}/{playerName}/edit", withPlayerAuth(makeHTTPHandleFunc(s.handleEditGameMode)))

	// Game endpoints
	router.HandleFunc("/games/{lobbyCode}/{playerName}/game", withPlayerAuth(makeHTTPHandleFunc(s.handleGame)))
	router.HandleFunc("/games/{lobbyCode}/{playerName}/combinations", withPlayerAuth(makeHTTPHandleFunc(s.handleCombination)))
	router.HandleFunc("/games/{lobbyCode}/{playerName}/words", withPlayerAuth(makeHTTPHandleFunc(s.handleGetWords)))
	router.HandleFunc("/games/{lobbyCode}/{playerName}/end", withPlayerAuth(makeHTTPHandleFunc(s.handleManualGameEnd)))

	// Events
	router.HandleFunc("/events", s.SSEHandler)
	router.HandleFunc("/broadcast", s.Broadcast)

	// Test
	router.HandleFunc("/test-ch/{channelID}", s.PublishToChannel)

	// Swagger
	router.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)
	return nil
}

func (s *APIServer) Run() {
	s.RegisterRoutes()
	router := s.router
	log.Fatal(http.ListenAndServe(s.listenAddr, router))

}
