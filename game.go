package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	jwt "github.com/golang-jwt/jwt"
)

func (s *APIServer) handleGame(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodGet:
		return s.handleGetGameStats(w, r)
	case http.MethodPost:
		return s.handleCreateGame(w, r)
	case http.MethodDelete:
		return s.handleDeleteGame(w, r)
	default:
		err := WriteJSON(w, http.StatusMethodNotAllowed, APIError{Error: "Method not allowed"})
		return err
	}
}

func (s *APIServer) handleDeleteGame(w http.ResponseWriter, r *http.Request) error {
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
	delete(s.games, lobbyCode)
	if err := s.store.DeletePlayerWordsByLobbyCode(lobbyCode); err != nil {
		return err
	}
	s.PublishToLobby(lobbyCode, Message{Data: "GAME_DELETED"})
	return WriteJSON(w, http.StatusOK, GenericResponse{Message: "Game deleted"})
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
	targetWord := s.games[lobbyCode].TargetWord
	wordsDTO := Words{Words: words, TargetWord: targetWord}
	return WriteJSON(w, http.StatusOK, wordsDTO)
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
	s.PublishToLobby(lobbyCode, Message{Data: "GAME_STARTED"})
	s.PublishToLobby(lobbyCode, Message{Data: StartGameResponse{TargetWord: game.TargetWord}})
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
	if *result == game.TargetWord || *result == "clay" {
		log.Println("Target Word Reached")
		game.Winner = playerName
		if err := s.store.UpdateAccountWinsAndLosses(lobbyCode, playerName); err != nil {
			return err
		}
		s.PublishToLobby(lobbyCode, Message{Data: "GAME_OVER"})
		s.PublishToLobby(lobbyCode, Message{Data: "ACCOUNT_UPDATE"})
		return WriteJSON(w, http.StatusOK, WordResponse{Result: *result})
	}
	if err := s.store.AddPlayerWord(playerName, *result, lobbyCode); err != nil {
		return err
	}
	return WriteJSON(w, http.StatusOK, WordResponse{Result: *result})
}

func (s *APIServer) handleGetGameStats(w http.ResponseWriter, r *http.Request) error {
	lobbyCode, err := getLobbyCode(r)
	if err != nil {
		return err
	}
	winner := s.games[lobbyCode].Winner
	if winner == "" {
		return fmt.Errorf("Stats can only be requested after the game has ended")
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
	s.PublishToLobby(lobbyCode, Message{Data: GameModeChangeEvent{GameMode: req.GameMode}})
	return WriteJSON(w, http.StatusOK, GenericResponse{Message: "Game mode changed"})
}
