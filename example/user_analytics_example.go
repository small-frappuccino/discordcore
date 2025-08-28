package example

// import (
// 	"fmt"
// 	"sync"
// 	"time"
// )

// // UserAnalyticsProcessor demonstrates how to build comprehensive user analytics
// // This example shows how to track:
// // - User join/leave events
// // - Time spent in server
// // - Account age analysis
// // - User retention metrics
// type UserAnalyticsProcessor struct {
// 	mu         sync.RWMutex
// 	userData   map[string]*UserSession // userID -> session data
// 	guildStats *GuildStatistics
// }

// // UserSession tracks individual user session data
// type UserSession struct {
// 	UserID         string
// 	Username       string
// 	GuildID        string
// 	JoinTime       time.Time
// 	LastSeen       time.Time
// 	TotalTimeSpent time.Duration
// 	AccountAge     time.Duration
// 	IsOnline       bool
// }

// // GuildStatistics tracks aggregate guild metrics
// type GuildStatistics struct {
// 	TotalUsers         int
// 	ActiveUsers        int
// 	AverageSessionTime time.Duration
// 	NewUsersToday      int
// 	UsersLeftToday     int
// 	AverageAccountAge  time.Duration
// }

// // NewUserAnalyticsProcessor creates a new analytics processor
// func NewUserAnalyticsProcessor() *UserAnalyticsProcessor {
// 	return &UserAnalyticsProcessor{
// 		userData: make(map[string]*UserSession),
// 		guildStats: &GuildStatistics{
// 			AverageSessionTime: 0,
// 			AverageAccountAge:  0,
// 		},
// 	}
// }

// // ProcessEvent handles different types of Discord events
// func (uap *UserAnalyticsProcessor) ProcessEvent(event Event) {
// 	uap.mu.Lock()
// 	defer uap.mu.Unlock()

// 	switch event.GetEventType() {
// 	case "guild_member_add":
// 		uap.handleUserJoin(event)
// 	case "guild_member_remove":
// 		uap.handleUserLeave(event)
// 	case "presence_update":
// 		uap.handlePresenceUpdate(event)
// 	case "guild_member_update":
// 		uap.handleUserUpdate(event)
// 	}
// }

// // handleUserJoin processes when a user joins the guild
// func (uap *UserAnalyticsProcessor) handleUserJoin(event Event) {
// 	data := event.GetData()
// 	userID := event.GetUserID()
// 	guildID := event.GetGuildID()

// 	// Calculate account age
// 	accountAge := time.Duration(0)
// 	if accountAgeDays, ok := data["account_age_days"].(float64); ok {
// 		accountAge = time.Duration(accountAgeDays*24) * time.Hour
// 	}

// 	session := &UserSession{
// 		UserID:     userID,
// 		Username:   data["username"].(string),
// 		GuildID:    guildID,
// 		JoinTime:   time.Now(),
// 		LastSeen:   time.Now(),
// 		IsOnline:   true,
// 		AccountAge: accountAge,
// 	}

// 	uap.userData[userID] = session
// 	uap.guildStats.TotalUsers++
// 	uap.guildStats.NewUsersToday++

// 	fmt.Printf("📈 User %s joined! Account age: %v\n",
// 		session.Username, session.AccountAge.Round(time.Hour))
// }

// // handleUserLeave processes when a user leaves the guild
// func (uap *UserAnalyticsProcessor) handleUserLeave(event Event) {
// 	userID := event.GetUserID()

// 	if session, exists := uap.userData[userID]; exists {
// 		timeInGuild := time.Since(session.JoinTime)
// 		session.TotalTimeSpent += timeInGuild

// 		uap.guildStats.TotalUsers--
// 		uap.guildStats.UsersLeftToday++

// 		fmt.Printf("📉 User %s left after %v in the server\n",
// 			session.Username, timeInGuild.Round(time.Minute))

// 		delete(uap.userData, userID)
// 	}
// }

// // handlePresenceUpdate processes presence changes
// func (uap *UserAnalyticsProcessor) handlePresenceUpdate(event Event) {
// 	data := event.GetData()
// 	userID := event.GetUserID()
// 	status := data["status"].(string)

// 	if session, exists := uap.userData[userID]; exists {
// 		session.LastSeen = time.Now()

// 		if status == "online" && !session.IsOnline {
// 			session.IsOnline = true
// 			uap.guildStats.ActiveUsers++
// 		} else if status != "online" && session.IsOnline {
// 			session.IsOnline = false
// 			uap.guildStats.ActiveUsers--
// 		}
// 	}
// }

// // handleUserUpdate processes user profile updates
// func (uap *UserAnalyticsProcessor) handleUserUpdate(event Event) {
// 	data := event.GetData()
// 	userID := event.GetUserID()

// 	if session, exists := uap.userData[userID]; exists {
// 		if username, ok := data["username"].(string); ok {
// 			session.Username = username
// 		}
// 	}
// }

// // GetGuildStats returns current guild statistics
// func (uap *UserAnalyticsProcessor) GetGuildStats() GuildStatistics {
// 	uap.mu.RLock()
// 	defer uap.mu.RUnlock()

// 	// Calculate averages
// 	totalTime := time.Duration(0)
// 	totalAge := time.Duration(0)
// 	activeSessions := 0

// 	for _, session := range uap.userData {
// 		totalTime += session.TotalTimeSpent
// 		totalAge += session.AccountAge
// 		if session.IsOnline {
// 			activeSessions++
// 		}
// 	}

// 	if len(uap.userData) > 0 {
// 		uap.guildStats.AverageSessionTime = totalTime / time.Duration(len(uap.userData))
// 		uap.guildStats.AverageAccountAge = totalAge / time.Duration(len(uap.userData))
// 	}
// 	uap.guildStats.ActiveUsers = activeSessions

// 	return *uap.guildStats
// }

// // GetUserSession returns session data for a specific user
// func (uap *UserAnalyticsProcessor) GetUserSession(userID string) (*UserSession, bool) {
// 	uap.mu.RLock()
// 	defer uap.mu.RUnlock()

// 	session, exists := uap.userData[userID]
// 	if !exists {
// 		return nil, false
// 	}

// 	// Return a copy to avoid race conditions
// 	sessionCopy := *session
// 	return &sessionCopy, true
// }

// // Start initializes the processor
// func (uap *UserAnalyticsProcessor) Start() {
// 	fmt.Println("🔍 User Analytics Processor started")
// }

// // Stop cleans up resources
// func (uap *UserAnalyticsProcessor) Stop() {
// 	fmt.Println("🔍 User Analytics Processor stopped")
// }
