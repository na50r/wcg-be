package main

import (
	"encoding/json"
	"log"
	"net/http"

	jwt "github.com/golang-jwt/jwt"
	"github.com/gorilla/mux"
)

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func makeHTTPHandleFunc(f APIFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			WriteJSON(w, http.StatusBadRequest, APIError{Error: err.Error()})
		}
	}
}

type APIServer struct {
	listenAddr   string
	store        Storage
	broker       *Broker
	lobbyClients map[string]map[int]bool // Maps a lobby code to a SET of clients
	playerClient map[string]int          // Maps each player to a client
	games        map[string]*Game
}

func NewAPIServer(listenAddr string, store Storage) *APIServer {
	return &APIServer{
		listenAddr:   listenAddr,
		store:        store,
		broker:       NewBroker(),
		lobbyClients: make(map[string]map[int]bool),
		playerClient: make(map[string]int),
		games:        make(map[string]*Game),
	}
}

// ChatGPT Aided
// Reference 1: https://stackhawkwpc.wpcomstaging.com/golang-cors-guide-what-it-is-and-how-to-enable-it/ (Only sets first header)
// Reference 2: https://stackoverflow.com/questions/61238680/access-to-fetch-at-from-origin-http-localhost3000-has-been-blocked-by-cors (Sets additional headers)
// Reference 3: https://medium.com/@gaurang.m/allowing-cross-site-requests-in-your-gin-app-golang-1332543d91ed (Implement something similar with Gin)
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

func (s *APIServer) Run() {
	router := mux.NewRouter()
	router.Use(corsMiddleware)
	//Endpoints
	router.HandleFunc("/login", makeHTTPHandleFunc(s.handleLogin))
	router.HandleFunc("/logout", makeHTTPHandleFunc(s.handleLogout))

	router.HandleFunc("/accounts", makeHTTPHandleFunc(s.handleRegister))
	router.HandleFunc("/account/{username}", withJWTAuth(makeHTTPHandleFunc(s.handleAccount)))
	router.HandleFunc("/account/{username}/images", withJWTAuth(makeHTTPHandleFunc(s.handleGetImages)))
	router.HandleFunc("/account/{username}/lobby", withJWTAuth(makeHTTPHandleFunc(s.handleCreateLobby)))


	// Lobby Endpoints
	router.HandleFunc("/lobbies", makeHTTPHandleFunc(s.handleGetLobbies))
	router.HandleFunc("/lobbies/join", makeHTTPHandleFunc(s.handleJoinLobby))
	
	router.HandleFunc("/lobbies/{lobbyCode}/{playerName}", withLobbyAuth(makeHTTPHandleFunc(s.handleGetLobby)))
	router.HandleFunc("/lobbies/{lobbyCode}/{playerName}/leave", withLobbyAuth(makeHTTPHandleFunc(s.handleLeaveLobby)))
	router.HandleFunc("/lobbies/{lobbyCode}/{playerName}/game", withLobbyAuth(makeHTTPHandleFunc(s.handleGame)))
	router.HandleFunc("/lobbies/{lobbyCode}/{playerName}/edit", withLobbyAuth(makeHTTPHandleFunc(s.handleEditGameMode)))

	// Game endpoints
	router.HandleFunc("/games/{lobbyCode}/{playerName}/combinations", withLobbyAuth(makeHTTPHandleFunc(s.handleMove)))
	router.HandleFunc("/games/{lobbyCode}/{playerName}/words", withLobbyAuth(makeHTTPHandleFunc(s.handleGetWords)))

	// Events
	router.HandleFunc("/events", s.SSEHandler)
	router.HandleFunc("/broadcast", s.broker.Broadcast)
	log.Fatal(http.ListenAndServe(s.listenAddr, router))
}

// Authentication Middleware Adapted from Anthony GG's tutorial
func withJWTAuth(handlerFunc http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, tokenExists, err := getToken(r)
		if err != nil {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			return
		}
		if !tokenExists {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			return
		}

		username, err := getUsername(r)
		if err != nil {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			return
		}
		if username != claims["username"] {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			return
		}
		handlerFunc(w, r)
	}
}

func withLobbyAuth(handlerFunc http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, tokenExists, err := getToken(r)
		if err != nil {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			log.Println("Unauthorized (Outdated Token)", err)
			return
		}
		if !tokenExists {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			log.Println("Unauthorized (No Token)", err)
			return
		}

		lobbyCode, err := getLobbyCode(r)
		if err != nil {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			log.Println("Unauthorized (No Lobby Code)", err)
			return
		}
		playerName, err := getPlayername(r)
		if err != nil {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			log.Println("Unauthorized (No Player Name)", err)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			log.Println("Unauthorized (No Claims)", err)
			return
		}
		if lobbyCode != claims["lobbyCode"] || playerName != claims["playerName"] {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			log.Println("Unauthorized (Invalid Lobby Code or Player Name)", err)
			return
		}
		handlerFunc(w, r)
	}
}
