package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	jwt "github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
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
	listenAddr      string
	store           Storage
	broker          *Broker
	lobbies2clients map[string]map[int]bool
	players2clients map[string]int
	games           map[string]*Game
}

func NewAPIServer(listenAddr string, store Storage) *APIServer {
	return &APIServer{
		listenAddr:      listenAddr,
		store:           store,
		broker:          NewBroker(),
		lobbies2clients: make(map[string]map[int]bool),
		players2clients: make(map[string]int),
		games:           make(map[string]*Game),
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

	router.HandleFunc("/lobbies", makeHTTPHandleFunc(s.handleGetLobbies))
	router.HandleFunc("/lobbies/join", makeHTTPHandleFunc(s.handleJoinLobby))

	// Lobby Endpoints
	router.HandleFunc("/lobbies/{lobbyCode}/{playerName}", withLobbyAuth(makeHTTPHandleFunc(s.handleGetLobby)))
	router.HandleFunc("/lobbies/{lobbyCode}/{playerName}/leave", withLobbyAuth(makeHTTPHandleFunc(s.handleLeaveLobby)))
	router.HandleFunc("/lobbies/{lobbyCode}/{playerName}/game", withLobbyAuth(makeHTTPHandleFunc(s.handleCreateGame)))
	router.HandleFunc("/lobbies/{lobbyCode}/{playerName}/edit", withLobbyAuth(makeHTTPHandleFunc(s.handleEditGameMode)))

	// Game endpoints
	router.HandleFunc("/games/{lobbyCode}/{playerName}/combinations", withLobbyAuth(makeHTTPHandleFunc(s.handleMove)))
	router.HandleFunc("/games/{lobbyCode}/{playerName}/words", withLobbyAuth(makeHTTPHandleFunc(s.handleGetWords)))
	router.HandleFunc("/games/{lobbyCode}/{playerName}/end", withLobbyAuth(makeHTTPHandleFunc(s.handleGetEndGame)))

	// Events
	router.HandleFunc("/events/lobbies", s.SSEHandler)
	router.HandleFunc("/events/publish", s.broker.PublishEndpoint)
	log.Fatal(http.ListenAndServe(s.listenAddr, router))
}

func (s *APIServer) handleGetWords(w http.ResponseWriter, r *http.Request) error {
	lobbyCode, err := getLobbyCode(r)
	if err != nil {
		return err
	}
	playerName, err := getPlayername(r)
	if err != nil {
		return err
	}
	words, err := s.store.GetPlayerWords(playerName, lobbyCode)
	if err != nil {
		return err
	}
	return WriteJSON(w, http.StatusOK, words)
}

func (s *APIServer) handleCreateGame(w http.ResponseWriter, r *http.Request) error {
	token, tokenExists, err := getToken(r)
	if err != nil {
		return err
	}
	if !tokenExists {
		return fmt.Errorf("unauthorized")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return fmt.Errorf("unauthorized")
	}
	isOwner := claims["isOwner"].(bool)
	if !isOwner {
		return fmt.Errorf("unauthorized")
	}
	lobbyCode, err := getLobbyCode(r)
	if err != nil {
		return err
	}
	req := new(StartGameRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return err
	}
	if err := s.store.EditGameMode(lobbyCode, req.GameMode); err != nil {
		return err
	}
	game, err := s.store.NewGame(lobbyCode)
	if err != nil {
		return err
	}
	s.games[lobbyCode] = game
	if err := s.store.SeedPlayerWords(lobbyCode); err != nil {
		return err
	}
	resp := StartGameResponse{TargetWord: game.TargetWord}
	log.Println("---")
	log.Printf("Game created\nLobby code: %s\nTarget word: %s", lobbyCode, game.TargetWord)
	log.Println("---")
	s.PublishToClients(lobbyCode, Message{Data: "GAME_STARTED"})
	s.PublishToClients(lobbyCode, Message{Data: StartGameResponse{TargetWord: game.TargetWord}})
	return WriteJSON(w, http.StatusOK, resp)
}

func (s *APIServer) handleMove(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		err := WriteJSON(w, http.StatusMethodNotAllowed, APIError{Error: "Method not allowed"})
		return err
	}
	lobbyCode, err := getLobbyCode(r)
	if err != nil {
		return err
	}
	game := s.games[lobbyCode]
	if game == nil {
		return fmt.Errorf("game not found")
	}
	req := new(WordRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return err
	}
	result, err := s.store.GetCombination(req.A, req.B)
	if err != nil {
		return err
	}
	playerName, err := getPlayername(r)
	if err != nil {
		return err
	}
	log.Printf("Player %s played %s + %s = %s", playerName, req.A, req.B, *result)
	winnerMsg := map[string]string{"WINNER": playerName}
	if *result == game.TargetWord || *result == "clay" {
		log.Println("Target Word Reached")
		game.Winner = playerName
		s.PublishToClients(lobbyCode, Message{Data: "GAME_OVER"})
		s.PublishToClients(lobbyCode, Message{Data: winnerMsg})
		return WriteJSON(w, http.StatusOK, WordResponse{Result: *result})
	}
	if err := s.store.AddPlayerWord(playerName, *result, lobbyCode); err != nil {
		return err
	}
	return WriteJSON(w, http.StatusOK, WordResponse{Result: *result})
}

func (s *APIServer) handleGetEndGame(w http.ResponseWriter, r *http.Request) error {
	lobbyCode, err := getLobbyCode(r)
	if err != nil {
		return err
	}
	winner := s.games[lobbyCode].Winner
	if winner == "" {
		return fmt.Errorf("game not over")
	}
	playerWordCounts, err := s.store.GetWordCountByLobbyCode(lobbyCode)
	if err != nil {
		return err
	}
	playerWordsDTO := []*PlayerWordDTO{}
	for _, playerWordCount := range playerWordCounts {
		player, err := s.store.GetPlayerByLobbyCodeAndName(playerWordCount.PlayerName, lobbyCode)
		if err != nil {
			return err
		}
		img, err := s.store.GetImage(player.ImageName)
		if err != nil {
			return err
		}
		playerWordsDTO = append(playerWordsDTO, &PlayerWordDTO{PlayerName: player.Name, Image: img, WordCount: playerWordCount.WordCount})
	}
	return WriteJSON(w, http.StatusOK, GameEndResponse{Winner: winner, PlayerWords: playerWordsDTO})
}

func (s *APIServer) handleEditGameMode(w http.ResponseWriter, r *http.Request) error {
	token, tokenExists, err := getToken(r)
	if err != nil {
		return err
	}
	if !tokenExists {
		return fmt.Errorf("unauthorized")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return fmt.Errorf("unauthorized")
	}
	isOwner := claims["isOwner"].(bool)
	if !isOwner {
		return fmt.Errorf("unauthorized")
	}

	lobbyCode, err := getLobbyCode(r)
	if err != nil {
		return err
	}
	req := new(ChangeGameModeRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return err
	}
	s.PublishToClients(lobbyCode, Message{Data: GameModeChangeEvent{GameMode: req.GameMode}})
	return WriteJSON(w, http.StatusOK, GenericResponse{Message: "Game mode changed"})
}

func (s *APIServer) handleGetLobby(w http.ResponseWriter, r *http.Request) error {
	lobbyCode, err := getLobbyCode(r)
	if err != nil {
		return err
	}
	players, err := s.store.GetPlayersByLobbyCode(lobbyCode)
	if err != nil {
		return err
	}
	lobby, err := s.store.GetLobbyByCode(lobbyCode)
	if err != nil {
		return err
	}
	var ownerName string
	playersDTO := []*PlayerDTO{}
	for _, player := range players {
		img, err := s.store.GetImage(player.ImageName)
		if err != nil {
			return err
		}
		if player.IsOwner {
			ownerName = player.Name
		}
		playersDTO = append(playersDTO, &PlayerDTO{Name: player.Name, Image: img})
	}
	lobbyDTO := NewLobbyDTO(lobby, ownerName, playersDTO)
	return WriteJSON(w, http.StatusOK, lobbyDTO)
}

func (s *APIServer) handleLeaveLobby(w http.ResponseWriter, r *http.Request) error {
	lobbyCode, err := getLobbyCode(r)
	if err != nil {
		return err
	}
	playerName, err := getPlayername(r)
	if err != nil {
		return err
	}
	player, err := s.store.GetPlayerByLobbyCodeAndName(playerName, lobbyCode)
	if err != nil {
		return err
	}
	if player.IsOwner {
		if err := s.store.DeleteLobby(lobbyCode); err != nil {
			return err
		}
		if err := s.store.DeletePlayersForLobby(lobbyCode); err != nil {
			return err
		}
		if err := s.store.DeletePlayerWordsByLobbyCode(lobbyCode); err != nil {
			return err
		}
		s.broker.Publish(Message{Data: "LOBBY_DELETED"})
		return WriteJSON(w, http.StatusOK, GenericResponse{Message: "Lobby deleted"})
	}
	if err := s.store.DeletePlayer(playerName, lobbyCode); err != nil {
		return err
	}
	if err := s.store.DeletePlayerWordsByPlayerAndLobbyCode(playerName, lobbyCode); err != nil {
		return err
	}
	delete(s.lobbies2clients[lobbyCode], s.players2clients[playerName])
	s.broker.Publish(Message{Data: "PLAYER_LEFT"})
	return WriteJSON(w, http.StatusOK, GenericResponse{Message: "Left Lobby"})
}

func (s *APIServer) handleJoinLobby(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		err := WriteJSON(w, http.StatusMethodNotAllowed, APIError{Error: "Method not allowed"})
		return err
	}
	var hasAccount = false
	_, tokenExists, err := getToken(r)
	if err != nil {
		return err
	}
	if tokenExists {
		hasAccount = true
	}

	req := new(JoinLobbyRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return err
	}
	imageName := s.store.NewImageForUsername(req.PlayerName)
	var player *Player
	if hasAccount {
		player, err = s.store.GetPlayerForAccount(req.PlayerName)
		if err != nil {
			return err
		}
		player.LobbyCode = req.LobbyCode
	} else {
		player = NewPlayer(req.PlayerName, req.LobbyCode, imageName, false, false)
	}
	if err := s.store.AddPlayerToLobby(req.LobbyCode, player); err != nil {
		return err
	}
	playerToken, err := createLobbyToken(player)
	if err != nil {
		return err
	}
	lobby, err := s.store.GetLobbyByCode(req.LobbyCode)
	if err != nil {
		return err
	}
	lobbyDTO := NewLobbyDTO(lobby, player.Name, []*PlayerDTO{})
	s.broker.Publish(Message{Data: "PLAYER_JOINED"})
	return WriteJSON(w, http.StatusOK, JoinLobbyRespone{Token: playerToken, LobbyDTO: *lobbyDTO})
}

func (s *APIServer) Play(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func (s *APIServer) handleCreateLobby(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		err := WriteJSON(w, http.StatusMethodNotAllowed, APIError{Error: "Method not allowed"})
		return err
	}
	req := new(CreateLobbyRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return err
	}
	username, err := getUsername(r)
	if err != nil {
		return err
	}
	owner, err := s.store.GetPlayerForAccount(username)
	if err != nil {
		return err
	}
	lobbyName := req.Name
	lobbyCode := uuid.New().String()[:6]
	lobby := NewLobby(lobbyName, lobbyCode, owner.ImageName)
	if err := s.store.CreateLobby(lobby); err != nil {
		return err
	}
	owner.LobbyCode = lobbyCode
	owner.IsOwner = true
	if err := s.store.CreatePlayer(owner); err != nil {
		return err
	}

	img, err := s.store.GetImage(owner.ImageName)
	if err != nil {
		return err
	}
	ownerDTO := &PlayerDTO{Name: owner.Name, Image: img}
	playersDTO := []*PlayerDTO{ownerDTO}
	lobbyDTO := NewLobbyDTO(lobby, owner.Name, playersDTO)
	token, err := createLobbyToken(owner)
	if err != nil {
		return err
	}
	resp := CreateLobbyResponse{Token: token, LobbyDTO: *lobbyDTO}
	s.broker.Publish(Message{Data: "LOBBY_CREATED"})
	return WriteJSON(w, http.StatusOK, resp)
}

func (s *APIServer) handleGetLobbies(w http.ResponseWriter, r *http.Request) error {
	lobbies, err := s.store.GetLobbies()
	if err != nil {
		return err
	}
	lobbiesDTO := []*LobbiesDTO{}
	for _, lobby := range lobbies {
		img, err := s.store.GetImage(lobby.ImageName)
		if err != nil {
			return err
		}
		lobby := &LobbiesDTO{Image: img, PlayerCount: lobby.PlayerCount, LobbyCode: lobby.LobbyCode}
		lobbiesDTO = append(lobbiesDTO, lobby)
	}
	return WriteJSON(w, http.StatusOK, lobbiesDTO)
}

func (s *APIServer) handleGetImages(w http.ResponseWriter, r *http.Request) error {
	images, err := s.store.GetImages()
	if err != nil {
		return err
	}
	names := make([]string, 0, len(images))
	for _, image := range images {
		names = append(names, image.Name)
	}
	resp := ImagesResponse{Names: names}
	return WriteJSON(w, http.StatusOK, resp)
}

func (s *APIServer) handleAccount(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodGet:
		return s.handleGetAccount(w, r)
	case http.MethodPut:
		return s.handleEditAccount(w, r)
	default:
		err := WriteJSON(w, http.StatusMethodNotAllowed, APIError{Error: "Method not allowed"})
		return err
	}
}

func (s *APIServer) handleEditAccount(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPut {
		err := WriteJSON(w, http.StatusMethodNotAllowed, APIError{Error: "Method not allowed"})
		return err
	}
	req := new(EditAccountRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return err
	}
	username, err := getUsername(r)
	if err != nil {
		return err
	}
	acc, err := s.store.GetAccountByUsername(username)
	if err != nil {
		return err
	}
	var msg string
	if req.Type == "PASSWORD" {
		if err := bcrypt.CompareHashAndPassword([]byte(acc.Password), []byte(req.OldPassword)); err != nil {
			return fmt.Errorf("Incorrect password, please try again")
		}
		if err := passwordValid(req.NewPassword); err != nil {
			return err
		}
		encpw, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		acc.Password = string(encpw)
		msg = "Password changed"
	}
	if req.Type == "USERNAME" {
		acc.Username = req.Username
		msg = "Username changed"
	}
	if req.Type == "IMAGE" {
		acc.ImageName = req.ImageName
		msg = "Image changed"
	}
	if err := s.store.UpdateAccount(acc); err != nil {
		return err
	}
	return WriteJSON(w, http.StatusOK, GenericResponse{Message: msg})
}

func (s *APIServer) handleRegister(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		err := WriteJSON(w, http.StatusMethodNotAllowed, APIError{Error: "Method not allowed"})
		return err
	}
	req := new(RegisterRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return err
	}

	if err := passwordValid(req.Password); err != nil {
		return err
	}

	acc, err := NewAccount(req.Username, req.Password)
	if err != nil {
		return err
	}
	imageName := s.store.NewImageForUsername(acc.Username)
	acc.ImageName = imageName

	if err := s.store.CreateAccount(acc); err != nil {
		return WriteJSON(w, http.StatusConflict, APIError{Error: "Username taken, choose another one"})
	}
	return WriteJSON(w, http.StatusCreated, GenericResponse{Message: "Account created"})
}

func (s *APIServer) handleLogin(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		err := WriteJSON(w, http.StatusMethodNotAllowed, APIError{Error: "Method not allowed"})
		return err
	}
	req := new(LoginRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return err
	}
	acc, err := s.store.GetAccountByUsername(req.Username)
	if err != nil {
		return err
	}
	pw := req.Password
	encpw := acc.Password
	if err := bcrypt.CompareHashAndPassword([]byte(encpw), []byte(pw)); err != nil {
		return fmt.Errorf("Incorrect password, please try again")
	}

	tokenString, err := createJWT(acc)
	if err != nil {
		return err
	}
	acc.Status = "ONLINE"
	if err := s.store.UpdateAccount(acc); err != nil {
		return err
	}
	resp := LoginResponse{Token: tokenString}
	return WriteJSON(w, http.StatusOK, resp)
}

func (s *APIServer) handleLogout(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		err := WriteJSON(w, http.StatusMethodNotAllowed, APIError{Error: "Method not allowed"})
		return err
	}
	token, tokenExists, err := getToken(r)
	if err != nil {
		return err
	}
	if !tokenExists {
		return fmt.Errorf("unauthorized")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return fmt.Errorf("unauthorized")
	}
	username := claims["username"].(string)
	acc, err := s.store.GetAccountByUsername(username)
	if err != nil {
		return err
	}
	if acc.Status == "OFFLINE" {
		return WriteJSON(w, http.StatusBadRequest, APIError{Error: "Already logged out"})
	}
	acc.Status = "OFFLINE"
	if err := s.store.UpdateAccount(acc); err != nil {
		return err
	}

	// Delete Lobby of logged out owner
	lobbyCode, err := s.store.GetLobbyForOwner(username)
	if err != nil {
		return err
	}
	if lobbyCode != "" {
		if err := s.store.DeleteLobby(lobbyCode); err != nil {
			return err
		}
		if err := s.store.DeletePlayersForLobby(lobbyCode); err != nil {
			return err
		}
		if err := s.store.DeletePlayerWordsByLobbyCode(lobbyCode); err != nil {
			return err
		}
		s.broker.Publish(Message{Data: "LOBBY_DELETED"})
	}
	return WriteJSON(w, http.StatusOK, GenericResponse{Message: "Logout successful"})
}

func (s *APIServer) handleGetAccount(w http.ResponseWriter, r *http.Request) error {
	username, err := getUsername(r)
	if err != nil {
		return err
	}
	acc, err := s.store.GetAccountByUsername(username)
	if err != nil {
		return err
	}
	img, err := s.store.GetImage(acc.ImageName)
	if err != nil {
		return err
	}

	resp := new(AccountDTO)
	resp.Username = acc.Username
	resp.Image = img
	resp.ImageName = acc.ImageName
	resp.CreatedAt = acc.CreatedAt
	resp.Wins = acc.Wins
	resp.Losses = acc.Losses
	resp.Status = acc.Status
	return WriteJSON(w, http.StatusOK, resp)
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
