package constants

type EventMesage string
type GameMode string
type Status string 
type Achievement string

const (
	LOBBY_CREATED EventMesage = "LOBBY_CREATED"
	PLAYER_JOINED EventMesage = "PLAYER_JOINED"
	GAME_STARTED  EventMesage = "GAME_STARTED"
	GAME_DELETED  EventMesage = "GAME_DELETED"
	LOBBY_DELETED EventMesage = "LOBBY_DELETED"
	PLAYER_LEFT   EventMesage = "PLAYER_LEFT"
	GAME_OVER     EventMesage = "GAME_OVER"
	ACCOUNT_UPDATE EventMesage = "ACCOUNT_UPDATE"
	WOMBO_COMBO_EVENT   EventMesage = "WOMBO_COMBO"
	TIMER_STOPPED EventMesage = "TIMER_STOPPED"
)

const (
	VANILLA GameMode = "Vanilla"
	WOMBO_COMBO GameMode = "Wombo Combo"
	FUSION_FRENZY GameMode = "Fusion Frenzy"
	DAILY_CHALLENGE GameMode = "Daily Challenge"
)

const (
	ONLINE Status = "ONLINE"
	OFFLINE Status = "OFFLINE"
)

const (
	Unauthorized string = "You are not authorized to perform this action."
)

const (
	NewWordCount Achievement = "New Word Count"
	WordCount Achievement = "Word Count"
	TargetWord Achievement = "Target Word"
)