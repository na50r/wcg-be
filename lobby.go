package main

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)


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
	delete(s.lobbyClients[lobbyCode], s.playerClient[playerName])
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
