package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"github.com/google/uuid"
	c "github.com/na50r/wombo-combo-go-be/constants"
)

// handleGetLobby godoc
// @Summary Get a lobby
// @Description Get a lobby
// @Tags lobby
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param lobbyCode path string true "Lobby code"
// @Param playerName path string true "Player name"
// @Success 200 {object} LobbyDTO
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /lobbies/{lobbyCode}/{playerName} [get]
func (s *APIServer) handleGetLobby(w http.ResponseWriter, r *http.Request) error {
	lobbyCode, err := GetLobbyCode(r)
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

// handleLeaveLobby godoc
// @Summary Leave a lobby
// @Description Leave a lobby
// @Tags lobby
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param lobbyCode path string true "Lobby code"
// @Param playerName path string true "Player name"
// @Success 200 {object} GenericResponse
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /lobbies/{lobbyCode}/{playerName}/leave [post]
func (s *APIServer) handleLeaveLobby(w http.ResponseWriter, r *http.Request) error {
	lobbyCode, err := GetLobbyCode(r)
	if err != nil {
		return err
	}
	playerName, err := GetPlayername(r)
	if err != nil {
		return err
	}
	player, err := s.store.GetPlayerByLobbyCodeAndName(playerName, lobbyCode)
	if err != nil {
		return err
	}
	if player.IsOwner {
		game := s.games[lobbyCode]
		if game != nil {
			game.StopTimer()
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
		if err := s.store.SetIsOwner(playerName, false); err != nil {
			return err
		}
		s.broker.Publish(Message{Data: c.LOBBY_DELETED})
		return WriteJSON(w, http.StatusOK, GenericResponse{Message: "Lobby deleted"})
	}
	if err := s.store.DeletePlayer(playerName, lobbyCode); err != nil {
		return err
	}
	if err := s.store.DeletePlayerWordsByPlayerAndLobbyCode(playerName, lobbyCode); err != nil {
		return err
	}
	if err := s.store.IncrementPlayerCount(lobbyCode, -1); err != nil {
		return err
	}
	delete(s.broker.lobbyClients[lobbyCode], s.broker.playerClient[playerName])
	s.broker.Publish(Message{Data: c.PLAYER_LEFT})
	return WriteJSON(w, http.StatusOK, GenericResponse{Message: "Left Lobby"})
}

// handleJoinLobby godoc
// @Summary Join a lobby
// @Description Join a lobby
// @Tags lobby
// @Accept json
// @Produce json
// @Param lobby body JoinLobbyRequest true "Lobby to join"
// @Success 200 {object} JoinLobbyRespone
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /lobbies [put]
func (s *APIServer) handleJoinLobby(w http.ResponseWriter, r *http.Request) error {
	token, tokenExists := getToken(r)

	req := new(JoinLobbyRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return err
	}
	var player *Player
	if tokenExists {
		// Verify only if a Token is used, otherwise ignore
		log.Println("Token Exists, Verifying...")
		_, err := verifyAccountJWT(token)
		if err != nil {
			return err
		}
		player, err = s.store.GetPlayerForAccount(req.PlayerName)
		if err != nil {
			return err
		}
		player.LobbyCode = req.LobbyCode
	} else {
		imageName := s.store.NewImageForUsername(req.PlayerName)
		player = NewPlayer(req.PlayerName, req.LobbyCode, imageName, false, false, 0, 0)
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
	s.broker.Publish(Message{Data: c.PLAYER_JOINED})
	return WriteJSON(w, http.StatusOK, JoinLobbyRespone{Token: playerToken, LobbyDTO: *lobbyDTO})
}

func (s *APIServer) handleLobbies(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodGet:
		return s.handleGetLobbies(w, r)
	case http.MethodPost:
		return s.handleCreateLobby(w, r)
	case http.MethodPut:
		return s.handleJoinLobby(w, r)
	default:
		err := WriteJSON(w, http.StatusMethodNotAllowed, APIError{Error: "Method not allowed"})
		return err
	}
}

// handleCreateLobby godoc
// @Summary Create a lobby (requires account)
// @Description Create a lobby
// @Tags lobby
// @Accept json
// @Produce json
// @Param lobby body CreateLobbyRequest true "Lobby to create"
// @Security BearerAuth
// @Success 200 {object} CreateLobbyResponse
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /lobbies [post]
func (s *APIServer) handleCreateLobby(w http.ResponseWriter, r *http.Request) error {
	token, tokenExists := getToken(r)
	if !tokenExists {
		return fmt.Errorf("unauthorized")
	}
	accountClaims, err := verifyAccountJWT(token)
	if err != nil {
		return err
	}
	username := accountClaims.Username
	req := new(CreateLobbyRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return err
	}
	owner, err := s.store.GetPlayerForAccount(username)
	if err != nil {
		return err
	}
	if err := s.store.SetIsOwner(username, true); err != nil {
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
	lobbyToken, err := createLobbyToken(owner)
	if err != nil {
		return err
	}
	resp := CreateLobbyResponse{Token: lobbyToken, LobbyDTO: *lobbyDTO}
	s.broker.Publish(Message{Data: c.LOBBY_CREATED})
	return WriteJSON(w, http.StatusOK, resp)
}

// handleGetLobbies godoc
// @Summary Get all lobbies
// @Description Get all lobbies
// @Tags lobby
// @Accept json
// @Produce json
// @Success 200 {array} LobbiesDTO
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /lobbies [get]
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

// handleEditGameMode godoc
// @Summary Edit a game mode in the lobby (owner)
// @Description Edit a game mode in the lobby
// @Tags lobby
// @Accept json
// @Produce json
// @Param game body EditGameRequest true "Game mode to change to"
// @Security BearerAuth
// @Param lobbyCode path string true "Lobby code"
// @Param playerName path string true "Player name"
// @Success 200 {object} GenericResponse
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /lobbies/{lobbyCode}/{playerName}/edit [put]
func (s *APIServer) handleEditGameMode(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPut {
		err := WriteJSON(w, http.StatusMethodNotAllowed, APIError{Error: "Method not allowed"})
		return err
	}
	token, tokenExists := getToken(r)
	if !tokenExists {
		return fmt.Errorf("unauthorized")
	}
	playerClaims, err := verifyPlayerJWT(token)
	if err != nil {
		return err
	}
	if !playerClaims.IsOwner {
		return fmt.Errorf("unauthorized")
	}

	lobbyCode, err := GetLobbyCode(r)
	if err != nil {
		return err
	}
	req := new(EditGameRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return err
	}
	s.broker.PublishToLobby(lobbyCode, Message{Data: GameEditEvent{GameMode: req.GameMode, Duration: req.Duration}})
	return WriteJSON(w, http.StatusOK, GenericResponse{Message: "Game mode changed"})
}
