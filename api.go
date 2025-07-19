package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"unicode"

	jwt "github.com/golang-jwt/jwt"
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
	router.HandleFunc("/lobby/{lobbyID}/join", makeHTTPHandleFunc(s.handleJoinLobby))
	//router.HandleFunc("/account/{username}", makeHTTPHandleFunc(s.handleUpdateAccount))

	// Events
	router.HandleFunc("/events/lobbies", s.broker.SSEHandler)
	router.HandleFunc("/events/messages", s.broker.SSEHandler)
	router.HandleFunc("/events/publish", s.broker.PublishEndpoint)
	log.Fatal(http.ListenAndServe(s.listenAddr, router))

}

func (s *APIServer) handleJoinLobby(w http.ResponseWriter, r *http.Request) error {
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
	lobbyID := req.Name
	owner.LobbyID = lobbyID
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
	lobbyDTO := &LobbyDTO{Owner: *ownerDTO, Players: []*PlayerDTO{}}
	s.broker.Publish(sse.Message{Data: "LOBBY_CREATED"})
	return WriteJSON(w, http.StatusOK, lobbyDTO)
}

func (s *APIServer) handleGetLobbies(w http.ResponseWriter, r *http.Request) error {
	owners, err := s.store.GetOwners()
	if err != nil {
		return err
	}
	lobbies := []*LobbiesDTO{}
	for _, owner := range owners {
		img, err := s.store.GetImage(owner.ImageName)
		if err != nil {
			return err
		}
		ownerDTO := &PlayerDTO{Name: owner.Name, Image: img}
		lobby := &LobbiesDTO{Owner: *ownerDTO, PlayerCount: 1, LobbyID: owner.LobbyID}
		lobbies = append(lobbies, lobby)
	}
	return WriteJSON(w, http.StatusOK, lobbies)
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

	// Delete Lobby of logged out users
	lobbyID, err := s.store.GetLobbyForOwner(username)
	if err != nil {
		return err
	}
	if err := s.store.DeleteLobby(lobbyID); err != nil {
		return err
	}
	s.broker.Publish(sse.Message{Data: "LOBBY_DELETED"})
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

func createJWT(account *Account) (string, error) {
	claims := &jwt.MapClaims{
		"exp":      time.Now().Add(time.Hour * 12).Unix(),
		"username": account.Username,
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
