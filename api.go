package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"unicode"

	jwt "github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/na50r/wombo-combo-go-be/sse"
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
	listenAddr string
	store      Storage
	broker     *sse.Broker
}

func NewAPIServer(listenAddr string, store Storage) *APIServer {
	return &APIServer{
		listenAddr: listenAddr,
		store:      store,
		broker:     sse.NewBroker(),
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
	router.HandleFunc("/accounts", makeHTTPHandleFunc(s.handleRegister))
	router.HandleFunc("/account/{username}", withJWTAuth(makeHTTPHandleFunc(s.handleGetAccount)))
	router.HandleFunc("/login", makeHTTPHandleFunc(s.handleLogin))
	router.HandleFunc("/logout", makeHTTPHandleFunc(s.handleLogout))
	router.HandleFunc("/account/{username}/images", withJWTAuth(makeHTTPHandleFunc(s.handleGetImages)))
	router.HandleFunc("/account/{username}/image", withJWTAuth(makeHTTPHandleFunc(s.handleChangeImage)))
	router.HandleFunc("/account/{username}/lobby", withJWTAuth(makeHTTPHandleFunc(s.handleCreateLobby)))
	router.HandleFunc("/lobbies", makeHTTPHandleFunc(s.handleGetLobbies))
	router.HandleFunc("/lobbies/join", makeHTTPHandleFunc(s.handleJoinLobby))
	//router.HandleFunc("/account/{username}", makeHTTPHandleFunc(s.handleUpdateAccount))

	// Lobby Endpoints
	router.HandleFunc("/lobbies/{lobbyCode}/view/{playerName}", withLobbyAuth(makeHTTPHandleFunc(s.handleGetLobby)))
	router.HandleFunc("/lobbies/{lobbyCode}/leave/{playerName}", withLobbyAuth(makeHTTPHandleFunc(s.handleLeaveLobby)))
	router.HandleFunc("/lobbies/{lobbyCode}/games/{playerName}", withLobbyAuth(makeHTTPHandleFunc(s.Play)))
	router.HandleFunc("/lobbies/{lobbyCode}/edit/{playerName}", withLobbyAuth(makeHTTPHandleFunc(s.EditGame)))

	// Events
	router.HandleFunc("/events/lobbies", s.broker.SSEHandler)
	router.HandleFunc("/events/publish", s.broker.PublishEndpoint)
	log.Fatal(http.ListenAndServe(s.listenAddr, router))
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
	lobbyDTO := &LobbyDTO{Owner: ownerName, Players: playersDTO, Name: lobby.Name, GameMode: lobby.GameMode, LobbyCode: lobby.LobbyCode}
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
		s.broker.Publish(sse.Message{Data: "LOBBY_DELETED"})
		return WriteJSON(w, http.StatusOK, GenericResponse{Message: "Lobby deleted"})
	}
	if err := s.store.DeletePlayer(playerName, lobbyCode); err != nil {
		return err
	}
	s.broker.Publish(sse.Message{Data: "PLAYER_LEFT"})
	return WriteJSON(w, http.StatusOK, GenericResponse{Message: "Left Lobby"})
}


func (s *APIServer) handleJoinLobby(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		err := WriteJSON(w, http.StatusMethodNotAllowed, APIError{Error: "Method not allowed"})
		return err
	}
	var hasAccount = false
	token, err := retrieveTokenForLobbyJoin(r)
	if err != nil {
		return err
	}
	if len(token.Raw) > 0 {
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
		player.ImageName = imageName
		player.HasAccount = true
		player.IsOwner = false
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
	lobbyDTO := &LobbyDTO{LobbyCode: lobby.LobbyCode, Name: lobby.Name, GameMode: lobby.GameMode}

	s.broker.Publish(sse.Message{Data: "PLAYER_JOINED"})
	return WriteJSON(w, http.StatusOK, JoinLobbyRespone{Token: playerToken, LobbyDTO: *lobbyDTO})
}

func (s *APIServer) Play(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func (s *APIServer) EditGame(w http.ResponseWriter, r *http.Request) error {
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
	lobby := &Lobby{Name: lobbyName, ImageName: owner.ImageName, LobbyCode: lobbyCode, GameMode: "TBD", PlayerCount: 1}
	if err := s.store.CreateLobby(lobby); err != nil {
		return err
	}
	owner.LobbyCode = lobbyCode
	owner.IsOwner = true
	owner.HasAccount = true
	if err := s.store.CreatePlayer(owner); err != nil {
		return err
	}

	img, err := s.store.GetImage(owner.ImageName)
	if err != nil {
		return err
	}
	ownerDTO := &PlayerDTO{Name: owner.Name, Image: img}
	playersDTO := []*PlayerDTO{ownerDTO}
	lobbyDTO := &LobbyDTO{Owner: owner.Name, Players: playersDTO, Name: lobby.Name, GameMode: lobby.GameMode, LobbyCode: lobby.LobbyCode}
	token, err := createLobbyToken(owner)
	if err != nil {
		return err
	}
	resp := CreateLobbyResponse{Token: token, LobbyDTO: *lobbyDTO}
	s.broker.Publish(sse.Message{Data: "LOBBY_CREATED"})
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

func (s *APIServer) handleChangeImage(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		err := WriteJSON(w, http.StatusMethodNotAllowed, APIError{Error: "Method not allowed"})
		return err
	}
	req := new(ChangeImageRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return err
	}

	username, err := getUsername(r)
	acc, err := s.store.GetAccountByUsername(username)
	if err != nil {
		return err
	}
	log.Println("Changing image for", username, "to", req.ImageName)
	acc.ImageName = req.ImageName
	if err := s.store.UpdateAccount(acc); err != nil {
		return err
	}
	return WriteJSON(w, http.StatusOK, GenericResponse{Message: "Image changed, Reloading App..."})
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
	token, err := retrieveToken(r)
	if err != nil {
		return err
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
		s.broker.Publish(sse.Message{Data: "LOBBY_DELETED"})
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

// Authentication Middleware Adapted from Anthony GG's tutorial
func withJWTAuth(handlerFunc http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := retrieveToken(r)
		if err != nil {
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
		token, err := retrieveToken(r)
		if err != nil {
			WriteJSON(w, http.StatusUnauthorized, APIError{Error: "unauthorized"})
			log.Println("Unauthorized (Outdated Token)", err)
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

func createJWT(account *Account) (string, error) {
	claims := &jwt.MapClaims{
		"exp":      time.Now().Add(time.Hour * 12).Unix(),
		"username": account.Username,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(JWT_SECRET))
}

func createLobbyToken(player *Player) (string, error) {
	claims := &jwt.MapClaims{
		"exp":       time.Now().Add(time.Hour * 4).Unix(),
		"playerName":  player.Name,
		"lobbyCode": player.LobbyCode,
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

func retrieveToken(r *http.Request) (jwt.Token, error) {
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		return jwt.Token{}, fmt.Errorf("unauthorized")
	}

	token, err := parseJWT(tokenString)
	if err != nil && token != nil && !token.Valid {
		return jwt.Token{}, fmt.Errorf("unauthorized")
	}
	return *token, nil
}

func retrieveTokenForLobbyJoin(r *http.Request) (jwt.Token, error) {
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		// Anon player
		return jwt.Token{}, nil
	}

	token, err := parseJWT(tokenString)
	if err != nil && token != nil && !token.Valid {
		return jwt.Token{}, fmt.Errorf("unauthorized")
	}
	return *token, nil
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
