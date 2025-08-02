package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"golang.org/x/crypto/bcrypt"
	"sort"
)

// handleGetAccount godoc
// @Summary Get an account
// @Description Get an account
// @Tags account
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param username path string true "Username"
// @Success 200 {object} AccountDTO
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /account/{username} [get]
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

// handleGetImages godoc
// @Summary Get all potential profile pictures
// @Description Get all potential profile pictures
// @Tags account
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param username path string true "Username"
// @Success 200 {object} ImagesResponse
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /account/{username}/images [get]
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

// handleEditAccount godoc
// @Summary Edit an account
// @Description Edit an account
// @Tags account
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param account body EditAccountRequest true "Account to edit"
// @Param username path string true "Username"
// @Success 200 {object} GenericResponse
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /account/{username} [put]
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

// handleRegister godoc
// @Summary Register an account
// @Description Register an account
// @Tags account
// @Accept json
// @Produce json
// @Param account body RegisterRequest true "Account to register"
// @Success 201 {object} GenericResponse
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /accounts [post]
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

// handleLogin godoc
// @Summary Log in an account
// @Description Authenticates a user and returns a JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param login body LoginRequest true "Username and password"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /login [post]
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
	acc.Status = ONLINE
	if err := s.store.UpdateAccount(acc); err != nil {
		return err
	}
	resp := LoginResponse{Token: tokenString}
	return WriteJSON(w, http.StatusOK, resp)
}

// handleLogout godoc
// @Summary Log out an account
// @Description Logs out a user
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} GenericResponse
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /logout [post]
func (s *APIServer) handleLogout(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		err := WriteJSON(w, http.StatusMethodNotAllowed, APIError{Error: "Method not allowed"})
		return err
	}
	accountClaims := r.Context().Value(authKey{}).(*AccountClaims)
	acc, err := s.store.GetAccountByUsername(accountClaims.Username)
	if err != nil {
		return err
	}
	if acc.Status == OFFLINE {
		return WriteJSON(w, http.StatusBadRequest, APIError{Error: "Already logged out"})
	}
	acc.Status = OFFLINE
	if err := s.store.UpdateAccount(acc); err != nil {
		return err
	}

	// Delete Lobby of logged out owner
	lobbyCode, err := s.store.GetLobbyForOwner(accountClaims.Username)
	if err != nil {
		return err
	}
	if lobbyCode != "" {
		if game, ok := s.games[lobbyCode]; ok {
			game.StopTimer()
			delete(s.games, lobbyCode)
		}
		if err := s.store.DeleteLobby(lobbyCode); err != nil {
			return err
		}
		if err := s.store.DeletePlayersForLobby(lobbyCode); err != nil {
			return err
		}
		if err := s.store.DeletePlayerWordsByLobbyCode(lobbyCode); err != nil {
			return err
		}
		err = s.store.SetIsOwner(accountClaims.Username, false)
		if err != nil {
			return err
		}
		delete(s.lobbyClients, lobbyCode)
		delete(s.games, lobbyCode)
		s.PublishToLobby(lobbyCode, Message{Data: GAME_DELETED})
		s.Publish(Message{Data: LOBBY_DELETED})
	}
	return WriteJSON(w, http.StatusOK, GenericResponse{Message: "Logout successful"})
}

// handleLeaderboard godoc
// @Summary Get the leaderboard
// @Description Get the leaderboard
// @Tags 
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param username path string true "Username"
// @Success 200 {array} ChallengeEntryDTO
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /account/{username}/leaderboard [get]
func (s *APIServer) handleLeaderboard(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		err := WriteJSON(w, http.StatusMethodNotAllowed, APIError{Error: "Method not allowed"})
		return err
	}
	entries, err := s.store.GetChallengeEntries()
	if err != nil {
		return err
	}
	entriesDTO := []*ChallengeEntryDTO{}
	for _, entry := range entries {
		image, err := s.store.GetImageByUsername(entry.Username)
		if err != nil {
			return err
		}
		entriesDTO = append(entriesDTO, &ChallengeEntryDTO{WordCount: entry.WordCount, Username: entry.Username, Image: image})
	}
	sort.Slice(entriesDTO, func(i, j int) bool {
		return entriesDTO[i].WordCount < entriesDTO[j].WordCount
	})
	return WriteJSON(w, http.StatusOK, entriesDTO)
}