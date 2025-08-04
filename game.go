package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sort"
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

// handleDeleteGame godoc
// @Summary Delete a game (owner)
// @Description Delete a game
// @Tags game
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param lobbyCode path string true "Lobby code"
// @Param playerName path string true "Player name"
// @Success 200 {object} GenericResponse
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /games/{lobbyCode}/{playerName}/game [delete]
func (s *APIServer) handleDeleteGame(w http.ResponseWriter, r *http.Request) error {
	playerClaims := r.Context().Value(authKey{}).(*PlayerClaims)
	if !playerClaims.IsOwner {
		return fmt.Errorf("unauthorized")
	}

	lobbyCode, err := getLobbyCode(r)
	if err != nil {
		return err
	}
	if game, ok := s.games[lobbyCode]; ok {
		game.StopTimer()
		delete(s.games, lobbyCode)
	}
	if err := s.store.DeletePlayerWordsByLobbyCode(lobbyCode); err != nil {
		log.Printf("Error deleting player words before returning to lobby: %v", err)
		return err
	}
	s.PublishToLobby(lobbyCode, Message{Data: GAME_DELETED})
	return WriteJSON(w, http.StatusOK, GenericResponse{Message: "Game deleted"})
}

// handleGetWords godoc
// @Summary Get a player's words
// @Description Get a player's words
// @Tags game
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param lobbyCode path string true "Lobby code"
// @Success 200 {object} Words
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /games/{lobbyCode}/{playerName}/words [get]
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
	targetWord, err := s.store.GetPlayerTargetWord(playerName, lobbyCode)
	if err != nil {
		return err
	}
	wordsDTO := Words{Words: words, TargetWord: targetWord}
	return WriteJSON(w, http.StatusOK, wordsDTO)
}

// handleCreateGame godoc
// @Summary Start a game (owner)
// @Description Start a game
// @Tags game
// @Accept json
// @Produce json
// @Param game body StartGameRequest true "Game to start"
// @Security BearerAuth
// @Param lobbyCode path string true "Lobby code"
// @Param playerName path string true "Player name"
// @Success 200 {object} GenericResponse
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /games/{lobbyCode}/{playerName}/game [post]
func (s *APIServer) handleCreateGame(w http.ResponseWriter, r *http.Request) error {
	playerClaims := r.Context().Value(authKey{}).(*PlayerClaims)
	if !playerClaims.IsOwner {
		return fmt.Errorf(Unauthorized)
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
	lobby, err := s.store.GetLobbyByCode(lobbyCode)
	if err != nil {
		return err
	}
	if req.GameMode == DAILY_CHALLENGE {
		if lobby.PlayerCount > 1 || req.WithTimer {
			return fmt.Errorf("Daily challenge must be played solo and without a timer")
		}
	}
	game, err := NewGame(s.store, lobbyCode, req.GameMode, req.WithTimer, req.Duration)
	if err != nil {
		return err
	}
	s.games[lobbyCode] = game
	if err := s.store.DeletePlayerWordsByLobbyCode(lobbyCode); err != nil {
		log.Printf("Error deleting player words before game start: %v", err)
		return err
	}
	if err := s.store.ResetPlayerPoints(lobbyCode); err != nil {
		return err
	}
	if err := SeedPlayerWords(s.store, lobbyCode, game); err != nil {
		return err
	}
	log.Printf("Game created\nLobby code: %s", lobbyCode)
	log.Printf("Game mode: %s", game.GameMode)
	log.Printf("Timer: %v", game.WithTimer)
	log.Println("Target words: ", game.TargetWords)
	s.PublishToLobby(lobbyCode, Message{Data: GAME_STARTED})
	game.StartTimer(s)
	return WriteJSON(w, http.StatusOK, GenericResponse{Message: "Game started"})
}

// handleCombination godoc
// @Summary Make a move
// @Description Make a move
// @Tags game
// @Accept json
// @Produce json
// @Param move body WordRequest true "Move to make"
// @Security BearerAuth
// @Param lobbyCode path string true "Lobby code"
// @Param playerName path string true "Player name"
// @Success 200 {object} WordResponse
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /games/{lobbyCode}/{playerName}/combinations [post]
func (s *APIServer) handleCombination(w http.ResponseWriter, r *http.Request) error {
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
		return fmt.Errorf("Game not found")
	}
	req := new(WordRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return err
	}
	result, isNew, err := GetCombination(s.store, req.A, req.B)
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
	log.Printf("Player %s played %s + %s = %s", playerName, req.A, req.B, result)
	err = ProcessMove(s, game, player, result)
	if err != nil {
		return err
	}
	return WriteJSON(w, http.StatusOK, WordResponse{Result: result, IsNew: isNew})
}

// handleGetGameStats godoc
// @Summary Get game stats
// @Description Get game stats
// @Tags game
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param lobbyCode path string true "Lobby code"
// @Param playerName path string true "Player name"
// @Success 200 {object} GameEndResponse
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /games/{lobbyCode}/{playerName}/game [get]
func (s *APIServer) handleGetGameStats(w http.ResponseWriter, r *http.Request) error {
	lobbyCode, err := getLobbyCode(r)
	if err != nil {
		return err
	}
	winner := s.games[lobbyCode].Winner
	playerWordCounts, err := s.store.GetWordCountByLobbyCode(lobbyCode)
	if err != nil {
		return err
	}
	playerWordsDTO := []*PlayerResultDTO{}
	for _, playerWordCount := range playerWordCounts {
		player, err := s.store.GetPlayerByLobbyCodeAndName(playerWordCount.PlayerName, lobbyCode)
		if err != nil {
			return err
		}
		img, err := s.store.GetImage(player.ImageName)
		if err != nil {
			return err
		}
		playerWordsDTO = append(playerWordsDTO, &PlayerResultDTO{PlayerName: player.Name, Image: img, WordCount: playerWordCount.WordCount, Points: player.Points + playerWordCount.WordCount})
	}
	sort.Slice(playerWordsDTO, func(i, j int) bool {
		if playerWordsDTO[i].PlayerName == winner {
			return true
		}
		if playerWordsDTO[j].PlayerName == winner {
			return false
		}
		return playerWordsDTO[i].Points > playerWordsDTO[j].Points
	})
	return WriteJSON(w, http.StatusOK, GameEndResponse{Winner: winner, PlayerWords: playerWordsDTO, GameMode: s.games[lobbyCode].GameMode, ManualEnd: s.games[lobbyCode].ManualEnd})
}

// handleManualGameEnd godoc
// @Summary End a game (owner)
// @Description End a game
// @Tags game
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param lobbyCode path string true "Lobby code"
// @Param playerName path string true "Player name"
// @Success 200 {object} GenericResponse
// @Failure 400 {object} APIError
// @Failure 405 {object} APIError
// @Router /games/{lobbyCode}/{playerName}/end [post]
func (s *APIServer) handleManualGameEnd(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		err := WriteJSON(w, http.StatusMethodNotAllowed, APIError{Error: "Method not allowed"})
		return err
	}
	playerClaims := r.Context().Value(authKey{}).(*PlayerClaims)
	if !playerClaims.IsOwner {
		return fmt.Errorf(Unauthorized)
	}
	lobbyCode, err := getLobbyCode(r)
	if err != nil {
		return err
	}
	game := s.games[lobbyCode]
	if game == nil {
		return fmt.Errorf("Game not found")
	}
	game.StopTimer()
	winner, err := s.store.SelectWinnerByPoints(lobbyCode)
	if err != nil {
		return err
	}
	if err := s.store.UpdateAccountWinsAndLosses(lobbyCode, winner); err != nil {
		return err
	}
	s.PublishToLobby(lobbyCode, Message{Data: ACCOUNT_UPDATE})
	game.Winner = winner
	game.ManualEnd = true
	s.PublishToLobby(lobbyCode, Message{Data: GAME_OVER})
	return WriteJSON(w, http.StatusOK, GenericResponse{Message: "Game ended"})
}

// Game Logic
func (g *Game) SetTarget() (string, error) {
	if g.GameMode == VANILLA {
		return "", nil
	}
	if g.GameMode == WOMBO_COMBO {
		log.Println("Number of target words ", len(g.TargetWords))
		targetWord := g.TargetWords[rand.Intn(len(g.TargetWords))]
		return targetWord, nil
	}
	if g.GameMode == FUSION_FRENZY {
		return g.TargetWord, nil
	}
	if g.GameMode == DAILY_CHALLENGE {
		return g.TargetWord, nil
	}
	return "", fmt.Errorf("Game mode %s not found", g.GameMode)
}

func ProcessMove(server *APIServer, game *Game, player *Player, result string) error {
	if game.GameMode == FUSION_FRENZY && player.TargetWord == result {
		game.StopTimer()
		game.Winner = player.Name
		if err := server.store.UpdateAccountWinsAndLosses(game.LobbyCode, player.Name); err != nil {
			return err
		}
		server.PublishToLobby(game.LobbyCode, Message{Data: GAME_OVER})
		server.PublishToLobby(game.LobbyCode, Message{Data: ACCOUNT_UPDATE})
		return nil
	}
	if game.GameMode == WOMBO_COMBO && player.TargetWord == result {
		var newTargetWord string
		var err error
		for {
			newTargetWord, err = game.SetTarget()
			if err != nil {
				return err
			}
			if newTargetWord != player.TargetWord {
				break
			}
		}
		log.Printf("Player %s reached target word %s, new target word is %s", player.Name, player.TargetWord, newTargetWord)
		if err := server.store.SetPlayerTargetWord(player.Name, newTargetWord, game.LobbyCode); err != nil {
			return err
		}
		if err := server.store.IncrementPlayerPoints(player.Name, game.LobbyCode, 10); err != nil {
			return err
		}
		server.PublishToLobby(game.LobbyCode, Message{Data: WOMBO_COMBO_EVENT})
	}
	if game.GameMode == DAILY_CHALLENGE && player.TargetWord == result {
		wordCounts, err := server.store.GetWordCountByLobbyCode(game.LobbyCode)
		if err != nil {
			return err
		}
		wordCount := wordCounts[0].WordCount
		log.Printf("Player %s completed daily challenge with word count %d", player.Name, wordCount)
		if err := server.store.AddDailyChallengeEntry(wordCount+1, player.Name); err != nil {
			return err
		}
		server.PublishToLobby(game.LobbyCode, Message{Data: GAME_OVER})
		return nil
	}
	if err := server.store.AddPlayerWord(player.Name, result, game.LobbyCode); err != nil {
		return err
	}
	return nil
}

func (g *Game) StartTimer(s *APIServer) {
	if g.WithTimer {
		g.Timer.Start(s, g.LobbyCode, g)
	}
}

func (g *Game) StopTimer() {
	if g.WithTimer {
		g.Timer.Stop()
	}
}

func SeedPlayerWords(s Storage, lobbyCode string, game *Game) error {
	players, err := s.GetPlayersByLobbyCode(lobbyCode)
	if err != nil {
		return err
	}
	for _, player := range players {
		target, err := game.SetTarget()
		if err != nil {
			return err
		}
		if err := s.SetPlayerTargetWord(player.Name, target, lobbyCode); err != nil {
			return err
		}
		s.AddPlayerWord(player.Name, "fire", lobbyCode)
		s.AddPlayerWord(player.Name, "water", lobbyCode)
		s.AddPlayerWord(player.Name, "earth", lobbyCode)
		s.AddPlayerWord(player.Name, "wind", lobbyCode)
	}
	return nil
}