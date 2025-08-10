package game

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sort"
	c "github.com/na50r/wombo-combo-go-be/constants"
	dto "github.com/na50r/wombo-combo-go-be/dto"
	u "github.com/na50r/wombo-combo-go-be/utility"
	st "github.com/na50r/wombo-combo-go-be/storage"
	t "github.com/na50r/wombo-combo-go-be/token"
)

type Game struct {
	GameMode    c.GameMode `json:"gameMode"`
	LobbyCode   string   `json:"lobbyCode"`
	TargetWord  string   `json:"targetWord"`
	TargetWords []string `json:"targetWords"`
	Winner      string   `json:"winner"`
	WithTimer   bool     `json:"withTimer"`
	Timer       *Timer   `json:"timer"`
	ManualEnd   bool     `json:"manualEnd"`
}

type GameService struct {
	store st.Storage
	broker *GameBroker
	games map[string]*Game
	apiKey string
	achievements AchievementMaps
}

func NewGameService(store st.Storage, apiKey string) *GameService {
	return &GameService{
		store: store,
		broker: NewGameBroker(),
		games: make(map[string]*Game),
		apiKey: apiKey,
	}
}

func (s *GameService) HandleGame(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodGet:
		return s.handleGetGameStats(w, r)
	case http.MethodPost:
		return s.handleCreateGame(w, r)
	case http.MethodDelete:
		return s.handleDeleteGame(w, r)
	default:
		err := u.WriteJSON(w, http.StatusMethodNotAllowed, dto.APIError{Error: "Method not allowed"})
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
// @Success 200 {object} dto.GenericResponse
// @Failure 400 {object} dto.APIError
// @Failure 405 {object} dto.APIError
// @Router /games/{lobbyCode}/{playerName}/game [delete]
func (s *GameService) handleDeleteGame(w http.ResponseWriter, r *http.Request) error {
	playerClaims := r.Context().Value(t.AuthKey{}).(*t.PlayerClaims)
	if !playerClaims.IsOwner {
		return fmt.Errorf("unauthorized")
	}

	lobbyCode, err := u.GetLobbyCode(r)
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
	s.broker.PublishToLobby(lobbyCode, Message{Data: c.GAME_DELETED})
	return u.WriteJSON(w, http.StatusOK, dto.GenericResponse{Message: "Game deleted"})
}

// HandleGetWords godoc
// @Summary Get a player's words
// @Description Get a player's words
// @Tags game
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param lobbyCode path string true "Lobby code"
// @Success 200 {object} dto.Words
// @Failure 400 {object} dto.APIError
// @Failure 405 {object} dto.APIError
// @Router /games/{lobbyCode}/{playerName}/words [get]
func (s *GameService) HandleGetWords(w http.ResponseWriter, r *http.Request) error {
	lobbyCode, err := u.GetLobbyCode(r)
	if err != nil {
		return err
	}
	playerName, err := u.GetPlayername(r)
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
	wordsDTO := dto.Words{Words: words, TargetWord: targetWord}
	return u.WriteJSON(w, http.StatusOK, wordsDTO)
}

// handleCreateGame godoc
// @Summary Start a game (owner)
// @Description Start a game
// @Tags game
// @Accept json
// @Produce json
// @Param game body dto.StartGameRequest true "Game to start"
// @Security BearerAuth
// @Param lobbyCode path string true "Lobby code"
// @Param playerName path string true "Player name"
// @Success 200 {object} dto.GenericResponse
// @Failure 400 {object} dto.APIError
// @Failure 405 {object} dto.APIError
// @Router /games/{lobbyCode}/{playerName}/game [post]
func (s *GameService) handleCreateGame(w http.ResponseWriter, r *http.Request) error {
	playerClaims := r.Context().Value(t.AuthKey{}).(*t.PlayerClaims)
	if !playerClaims.IsOwner {
		return fmt.Errorf(c.Unauthorized)
	}

	lobbyCode, err := u.GetLobbyCode(r)
	if err != nil {
		return err
	}
	req := new(dto.StartGameRequest)
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
	if req.GameMode == c.DAILY_CHALLENGE {
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
	s.broker.PublishToLobby(lobbyCode, Message{Data: c.GAME_STARTED})
	game.StartTimer(s)
	return u.WriteJSON(w, http.StatusOK, dto.GenericResponse{Message: "Game started"})
}

// HandleCombination godoc
// @Summary Make a move
// @Description Make a move
// @Tags game
// @Accept json
// @Produce json
// @Param move body dto.WordRequest true "Move to make"
// @Security BearerAuth
// @Param lobbyCode path string true "Lobby code"
// @Param playerName path string true "Player name"
// @Success 200 {object} dto.WordResponse
// @Failure 400 {object} dto.APIError
// @Failure 405 {object} dto.APIError
// @Router /games/{lobbyCode}/{playerName}/combinations [post]
func (s *GameService) HandleCombination(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		err := u.WriteJSON(w, http.StatusMethodNotAllowed, dto.APIError{Error: "Method not allowed"})
		return err
	}
	lobbyCode, err := u.GetLobbyCode(r)
	if err != nil {
		return err
	}
	game := s.games[lobbyCode]
	if game == nil {
		return fmt.Errorf("Game not found")
	}
	req := new(dto.WordRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return err
	}
	result, isNew, err := st.GetCombination(s.store, req.A, req.B, s.apiKey)
	if err != nil {
		return err
	}
	playerName, err := u.GetPlayername(r)
	if err != nil {
		return err
	}
	player, err := s.store.GetPlayerByLobbyCodeAndName(playerName, lobbyCode)
	if err != nil {
		return err
	}
	log.Printf("Player %s played %s + %s = %s", playerName, req.A, req.B, result)
	err = ProcessMove(s, game, player, result, isNew)
	if err != nil {
		return err
	}
	return u.WriteJSON(w, http.StatusOK, dto.WordResponse{Result: result, IsNew: isNew})
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
// @Success 200 {object} dto.GameEndResponse
// @Failure 400 {object} dto.APIError
// @Failure 405 {object} dto.APIError
// @Router /games/{lobbyCode}/{playerName}/game [get]
func (s *GameService) handleGetGameStats(w http.ResponseWriter, r *http.Request) error {
	lobbyCode, err := u.GetLobbyCode(r)
	if err != nil {
		return err
	}
	winner := s.games[lobbyCode].Winner
	playerWordCounts, err := s.store.GetWordCountByLobbyCode(lobbyCode)
	if err != nil {
		return err
	}
	playerWordsDTO := []*dto.PlayerResultDTO{}
	for _, playerWordCount := range playerWordCounts {
		player, err := s.store.GetPlayerByLobbyCodeAndName(playerWordCount.PlayerName, lobbyCode)
		if err != nil {
			return err
		}
		img, err := s.store.GetImage(player.ImageName)
		if err != nil {
			return err
		}
		playerWordsDTO = append(playerWordsDTO, &dto.PlayerResultDTO{PlayerName: player.Name, Image: img, WordCount: playerWordCount.WordCount, Points: player.Points + playerWordCount.WordCount})
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
	return u.WriteJSON(w, http.StatusOK, dto.GameEndResponse{Winner: winner, PlayerWords: playerWordsDTO, GameMode: s.games[lobbyCode].GameMode, ManualEnd: s.games[lobbyCode].ManualEnd})
}

// HandleManualGameEnd godoc
// @Summary End a game (owner)
// @Description End a game
// @Tags game
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param lobbyCode path string true "Lobby code"
// @Param playerName path string true "Player name"
// @Success 200 {object} dto.GenericResponse
// @Failure 400 {object} dto.APIError
// @Failure 405 {object} dto.APIError
// @Router /games/{lobbyCode}/{playerName}/end [post]
func (s *GameService) HandleManualGameEnd(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		err := u.WriteJSON(w, http.StatusMethodNotAllowed, dto.APIError{Error: "Method not allowed"})
		return err
	}
	playerClaims := r.Context().Value(t.AuthKey{}).(*t.PlayerClaims)
	if !playerClaims.IsOwner {
		return fmt.Errorf(c.Unauthorized)
	}
	lobbyCode, err := u.GetLobbyCode(r)
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
	s.broker.PublishToLobby(lobbyCode, Message{Data: c.ACCOUNT_UPDATE})
	game.Winner = winner
	game.ManualEnd = true
	s.broker.PublishToLobby(lobbyCode, Message{Data: c.GAME_OVER})
	return u.WriteJSON(w, http.StatusOK, dto.GenericResponse{Message: "Game ended"})
}

// Game Logic
func (g *Game) SetTarget() (string, error) {
	if g.GameMode == c.VANILLA {
		return "", nil
	}
	if g.GameMode == c.WOMBO_COMBO {
		log.Println("Number of target words ", len(g.TargetWords))
		targetWord := g.TargetWords[rand.Intn(len(g.TargetWords))]
		return targetWord, nil
	}
	if g.GameMode == c.FUSION_FRENZY {
		return g.TargetWord, nil
	}
	if g.GameMode == c.DAILY_CHALLENGE {
		return g.TargetWord, nil
	}
	return "", fmt.Errorf("Game mode %s not found", g.GameMode)
}

func ProcessMove(server *GameService, game *Game, player *st.Player, result string, isNew bool) error {
	if game.GameMode == c.FUSION_FRENZY && player.TargetWord == result {
		game.StopTimer()
		game.Winner = player.Name
		if err := server.store.UpdateAccountWinsAndLosses(game.LobbyCode, player.Name); err != nil {
			return err
		}
		if err := server.store.UpdateAccountWordCount(player.Name, player.NewWordCount, player.WordCount); err != nil {
			return err
		}
		server.broker.PublishToLobby(game.LobbyCode, Message{Data: c.GAME_OVER})
		server.broker.PublishToLobby(game.LobbyCode, Message{Data: c.ACCOUNT_UPDATE})
		return nil
	}
	if game.GameMode == c.WOMBO_COMBO && player.TargetWord == result {
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
			server.broker.PublishToLobby(game.LobbyCode, Message{Data: c.WOMBO_COMBO_EVENT})
	}
	if game.GameMode == c.DAILY_CHALLENGE && player.TargetWord == result {
		wordCounts, err := server.store.GetWordCountByLobbyCode(game.LobbyCode)
		if err != nil {
			return err
		}
		wordCount := wordCounts[0].WordCount
		log.Printf("Player %s completed daily challenge with word count %d", player.Name, wordCount)
		if err := server.store.AddDailyChallengeEntry(wordCount+1, player.Name); err != nil {
			return err
		}
		server.broker.PublishToLobby(game.LobbyCode, Message{Data: c.GAME_OVER})
		return nil
	}
	if err := server.store.AddPlayerWord(player.Name, result, game.LobbyCode); err != nil {
		return err
	}
	updatedWordCnt := player.WordCount + 1
	updatedNewWordCnt := player.NewWordCount
	if isNew {
		updatedNewWordCnt = player.NewWordCount + 1
	}
	if err := server.store.UpdatePlayerWordCount(player.Name, game.LobbyCode, updatedNewWordCnt, updatedWordCnt); err != nil {
		return err
	}
	if err := CheckAchievements(server, player.Name, updatedWordCnt, updatedNewWordCnt, result); err != nil {
		return err
	}
	return nil
}

func (g *Game) StartTimer(s *GameService) {
	if g.WithTimer {
		g.Timer.Start(s, g.LobbyCode, g)
	}
}

func (g *Game) StopTimer() {
	if g.WithTimer {
		g.Timer.Stop()
	}
}

func SeedPlayerWords(s st.Storage, lobbyCode string, game *Game) error {
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

func NewGame(s st.Storage, lobbyCode string, gameMode c.GameMode, withTimer bool, duration int) (*Game, error) {
	game := new(Game)
	game.LobbyCode = lobbyCode
	game.GameMode = gameMode
	game.WithTimer = withTimer
	game.ManualEnd = false

	if withTimer {
		game.Timer = NewTimer(duration)
	}

	err := fmt.Errorf("Game mode %s not found", gameMode)
	if gameMode == c.VANILLA {
		return game, nil
	}
	// Reachability is between 0 and 1
	// Reachability is computed with: 1 / (2 ^ depth)
	// Reachability is updated with:
	// 0.75 * newReachability + 0.25 * oldReachability if newDepth < oldDepth
	// 0.25 * newReachability + 0.75 * oldReachability if newDepth >= oldDepth
	// The less deep and the more paths are available, the more reachable a word is
	if gameMode == c.FUSION_FRENZY {
		game.TargetWord, err = s.GetTargetWord(0.0375, 0.2, 10)
		if err != nil {
			return nil, err
		}
		return game, nil
	}
	if gameMode == c.WOMBO_COMBO {
		game.TargetWords, err = s.GetTargetWords(0.0375, 0.2, 10)
		if err != nil {
			return nil, err
		}
		return game, nil
	}
	if gameMode == c.DAILY_CHALLENGE {
		game.TargetWord, err = s.CreateOrGetDailyWord(0.0375, 0.2, 8)
		if err != nil {
			log.Printf("Error creating or getting daily word: %v", err)
			return nil, err
		}
		return game, nil
	}
	return nil, err
}


func (s *GameService) Logout(lobbyCode, username string) error {
	var err error
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
		err = s.store.SetIsOwner(username, false)
		if err != nil {
			return err
		}
		delete(s.broker.lobbyClients, lobbyCode)
		delete(s.games, lobbyCode)
		s.broker.PublishToLobby(lobbyCode,Message{Data: c.GAME_DELETED})
		s.broker.Publish(Message{Data: c.LOBBY_DELETED})
		return nil
}


// handleLeaderboard godoc
// @Summary Get the leaderboard
// @Description Get the leaderboard
// @Tags
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param username path string true "Username"
// @Success 200 {array} dto.ChallengeEntryDTO
// @Failure 400 {object} dto.APIError
// @Failure 405 {object} dto.APIError
// @Router /account/{username}/leaderboard [get]
func (s *GameService) HandleLeaderboard(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		err := u.WriteJSON(w, http.StatusMethodNotAllowed, dto.APIError{Error: "Method not allowed"})
		return err
	}
	entries, err := s.store.GetChallengeEntries()
	if err != nil {
		return err
	}
	entriesDTO := []*dto.ChallengeEntryDTO{}
	for _, entry := range entries {
		image, err := s.store.GetImageByUsername(entry.Username)
		if err != nil {
			return err
		}
		entriesDTO = append(entriesDTO, &dto.ChallengeEntryDTO{WordCount: entry.WordCount, Username: entry.Username, Image: image})
	}
	sort.Slice(entriesDTO, func(i, j int) bool {
		return entriesDTO[i].WordCount < entriesDTO[j].WordCount
	})
	return u.WriteJSON(w, http.StatusOK, entriesDTO)
}
