package main

import (
	"log"
	"strconv"
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
		case NewWordCount:
			val, err := strconv.Atoi(entry.Value)
			if err != nil {
				return err
			}
			newWordCnt[val] = entry.Title
		case WordCount:
			val, err := strconv.Atoi(entry.Value)
			if err != nil {
				return err
			}
			wordCnt[val] = entry.Title
		case TargetWord:
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
	}
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
	if title, ok := a.TargetWord[currentWord]; ok {
		err := UnlockAchievement(s, username, title)
		if err != nil {
			return err
		}
	}
	return nil
}
