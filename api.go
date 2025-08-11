package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"

	"fmt"

	"github.com/gorilla/mux"
	a "github.com/na50r/wombo-combo-go-be/account"
	c "github.com/na50r/wombo-combo-go-be/constants"
	dto "github.com/na50r/wombo-combo-go-be/dto"
	g "github.com/na50r/wombo-combo-go-be/game"
	st "github.com/na50r/wombo-combo-go-be/storage"
	t "github.com/na50r/wombo-combo-go-be/token"
	u "github.com/na50r/wombo-combo-go-be/utility"
	httpSwagger "github.com/swaggo/http-swagger"
	"golang.org/x/crypto/bcrypt"
)

// Allows error handling
type APIFunc func(http.ResponseWriter, *http.Request) error

func makeHTTPHandleFunc(f APIFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			u.WriteJSON(w, http.StatusBadRequest, dto.APIError{Error: err.Error()})
		}
	}
}

type APIServer struct {
	router       *mux.Router
	listenAddr   string
	store        st.Storage
	gameService  *g.GameService
	accountService *a.AccountService
}

func NewAPIServer(listenAddr string, store st.Storage) *APIServer {
	s := APIServer{
		router:     mux.NewRouter(),
		listenAddr: listenAddr,
		store:      store,
		accountService: a.NewAccountService(store),
		gameService: g.NewGameService(store, COHERE_API_KEY),
	}
	s.gameService.SetupAchievements()
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

	router.HandleFunc("/accounts", makeHTTPHandleFunc(s.accountService.HandleRegister))
	router.HandleFunc("/account/{username}", t.WithAccountAuth(makeHTTPHandleFunc(s.accountService.HandleAccount)))
	router.HandleFunc("/account/{username}/images", t.WithAccountAuth(makeHTTPHandleFunc(s.accountService.HandleGetImages)))

	//Account / Game intersection
	router.HandleFunc("/account/{username}/leaderboard", t.WithAccountAuth(makeHTTPHandleFunc(s.gameService.HandleLeaderboard)))
	router.HandleFunc("/account/{username}/achievements", t.WithAccountAuth(makeHTTPHandleFunc(s.gameService.HandleAchievements)))

	// Lobby Endpoints
	router.HandleFunc("/lobbies", makeHTTPHandleFunc(s.gameService.HandleLobbies))
	router.HandleFunc("/lobbies/{lobbyCode}/{playerName}", t.WithPlayerAuth(makeHTTPHandleFunc(s.gameService.HandleGetLobby)))
	router.HandleFunc("/lobbies/{lobbyCode}/{playerName}/leave", t.WithPlayerAuth(makeHTTPHandleFunc(s.gameService.HandleLeaveLobby)))
	router.HandleFunc("/lobbies/{lobbyCode}/{playerName}/edit", t.WithPlayerAuth(makeHTTPHandleFunc(s.gameService.HandleEditGameMode)))

	// Game endpoints
	router.HandleFunc("/games/{lobbyCode}/{playerName}/game", t.WithPlayerAuth(makeHTTPHandleFunc(s.gameService.HandleGame)))
	router.HandleFunc("/games/{lobbyCode}/{playerName}/combinations", t.WithPlayerAuth(makeHTTPHandleFunc(s.gameService.HandleCombination)))
	router.HandleFunc("/games/{lobbyCode}/{playerName}/words", t.WithPlayerAuth(makeHTTPHandleFunc(s.gameService.HandleGetWords)))
	router.HandleFunc("/games/{lobbyCode}/{playerName}/end", t.WithPlayerAuth(makeHTTPHandleFunc(s.gameService.HandleManualGameEnd)))

	// Events
	router.HandleFunc("/events", s.gameService.SSEHandler)
	router.HandleFunc("/broadcast", s.gameService.Broadcast)

	// Swagger
	router.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	// Health
	router.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong\n"))
	})


	// Testing
	router.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		size := 1024 * 1024;
		buf := bytes.Repeat([]byte("A"), size)
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})
	return nil
}

func (s *APIServer) Run() {
	s.RegisterRoutes()
	router := s.router
	log.Fatal(http.ListenAndServe(s.listenAddr, router))

}


// handleLogin godoc
// @Summary Log in an account
// @Description Authenticates a user and returns a JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param login body dto.LoginRequest true "Username and password"
// @Success 200 {object} dto.LoginResponse
// @Failure 400 {object} dto.APIError
// @Failure 405 {object} dto.APIError
// @Router /login [post]
func (s *APIServer) handleLogin(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		err := u.WriteJSON(w, http.StatusMethodNotAllowed, dto.APIError{Error: "Method not allowed"})
		return err
	}
	req := new(dto.LoginRequest)
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

	tokenString, err := t.CreateJWT(acc)
	if err != nil {
		return err
	}
	acc.Status = c.ONLINE
	if err := s.store.UpdateAccount(acc); err != nil {
		return err
	}
	resp := dto.LoginResponse{Token: tokenString}
	log.Printf("User %s logged in\n", acc.Username)
	return u.WriteJSON(w, http.StatusOK, resp)
}

// handleLogout godoc
// @Summary Log out an account
// @Description Logs out a user
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.GenericResponse
// @Failure 400 {object} dto.APIError
// @Failure 405 {object} dto.APIError
// @Router /logout [post]
func (s *APIServer) handleLogout(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		err := u.WriteJSON(w, http.StatusMethodNotAllowed, dto.APIError{Error: "Method not allowed"})
		return err
	}
	token, tokenExists := t.GetToken(r)
	if !tokenExists {
		return fmt.Errorf(c.Unauthorized)
	}

	accountClaims, err := t.VerifyAccountJWT(token)
	if err != nil {
		return err
	}
	acc, err := s.store.GetAccountByUsername(accountClaims.Username)
	if err != nil {
		return err
	}
	if acc.Status == c.OFFLINE {
		return u.WriteJSON(w, http.StatusBadRequest, dto.APIError{Error: "Already logged out"})
	}
	acc.Status = c.OFFLINE
	if err := s.store.UpdateAccount(acc); err != nil {
		return err
	}

	// Delete Lobby of logged out owner
	lobbyCode, err := s.store.GetLobbyForOwner(accountClaims.Username)
	if err != nil {
		return err
	}
	if lobbyCode != "" {
		if err := s.gameService.Logout(lobbyCode, accountClaims.Username); err != nil {
			return err
		}
	}
	log.Printf("User %s logged out\n", accountClaims.Username)
	return u.WriteJSON(w, http.StatusOK, dto.GenericResponse{Message: "Logout successful"})
}