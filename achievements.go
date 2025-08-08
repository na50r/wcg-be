package main

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	c "github.com/na50r/wombo-combo-go-be/constants"
)

type AchievementMaps struct {
	NewWordCount map[int]string // int milestone → achievement title
	WordCount    map[int]string
	TargetWord   map[string]string // word string → achievement title
}

// TODO: Figure out a neat way to achievements
// Requirement: Must be easily extensible
func (server *APIServer) SetupAchievements() error {
	s := server.store
	newWordCnt := map[int]string{}
	wordCnt := map[int]string{}
	targetWord := map[string]string{}
	achievementEntries, err := s.GetAchievements()
	if err != nil {
		return err
	}
	for _, entry := range achievementEntries {
		switch entry.Type {
		case c.NewWordCount:
			val, err := strconv.Atoi(entry.Value)
			if err != nil {
				return err
			}
			newWordCnt[val] = entry.Title
		case c.WordCount:
			val, err := strconv.Atoi(entry.Value)
			if err != nil {
				return err
			}
			wordCnt[val] = entry.Title
		case c.TargetWord:
			targetWord[entry.Value] = entry.Title
		}
	}
	server.achievements = AchievementMaps{
		NewWordCount: newWordCnt,
		WordCount:    wordCnt,
		TargetWord:   targetWord,
	}
	log.Println("Achievements loaded")
	return nil
}

func UnlockAchievement(s *APIServer, username, achievementTitle string) error {
	newUnlock, err := s.store.UnlockAchievement(username, achievementTitle)
	if err != nil {
		return err
	}
	if newUnlock {
		log.Printf("Achievement unlocked: %s", achievementTitle)
		s.broker.PublishToPlayer(username, Message{Data: AchievementEvent{AchievementTitle: achievementTitle}})
	}
	log.Printf("Achievement already unlocked: %s", achievementTitle)
	return nil
}

func CheckAchievements(s *APIServer, username string, updatedWordCnt, updatedNewWordCnt int, currentWord string) error {
	a := s.achievements
	if title, ok := a.NewWordCount[updatedNewWordCnt]; ok {
		err := UnlockAchievement(s, username, title)
		if err != nil {
			return err
		}
	}
	if title, ok := a.WordCount[updatedWordCnt]; ok {
		err := UnlockAchievement(s, username, title)
		if err != nil {
			return err
		}
	}
	if title, ok := a.TargetWord[strings.ToLower(currentWord)]; ok {
		err := UnlockAchievement(s, username, title)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetAchievementsForUser(s *APIServer, username string) ([]*AchievementDTO, error) {
	achievementTitles, err := s.store.GetAchievementsForUser(username)
	if err != nil {
		return nil, err
	}
	unlockedAchievements := map[string]bool{}
	for _, title := range achievementTitles {
		unlockedAchievements[title] = true
	}
	allAchievements, err := s.store.GetAchievements()
	if err != nil {
		return nil, err
	}
	achievements := []*AchievementDTO{}
	for _, entry := range allAchievements {
		image, err := s.store.GetAchievementImage(entry.ImageName)
		if err != nil {
			return nil, err
		}
		unlocked := unlockedAchievements[entry.Title]
		achievements = append(achievements, &AchievementDTO{Title: entry.Title, Description: entry.Description, Image: image, Unlocked: unlocked})
	}
	return achievements, nil
}

// handleAchievements godoc
// @Summary Get all achievements for a user
// @Description Get all achievements for a user
// @Tags account
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param username path string true "Username"
// @Success 200 {array} AchievementDTO
func (s *APIServer) handleAchievements(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		err := WriteJSON(w, http.StatusMethodNotAllowed, APIError{Error: "Method not allowed"})
		return err
	}
	username, err := GetUsername(r)
	if err != nil {
		return err
	}
	achievements, err := GetAchievementsForUser(s, username)
	if err != nil {
		return err
	}
	return WriteJSON(w, http.StatusOK, achievements)
}
