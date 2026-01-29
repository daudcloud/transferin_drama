package middleware

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/telebot.v3"
)

// CommandSpamProtection prevents users from spamming commands
type CommandSpamProtection struct {
	mu            sync.RWMutex
	lastCommand   map[int64]time.Time
	commandDelay  time.Duration
	warningCount  map[int64]int
	banUntil      map[int64]time.Time
	cleanupTicker *time.Ticker
}

var spamProtection *CommandSpamProtection

func init() {
	spamProtection = &CommandSpamProtection{
		lastCommand:  make(map[int64]time.Time),
		commandDelay: 2 * time.Second,
		warningCount: make(map[int64]int),
		banUntil:     make(map[int64]time.Time),
	}

	// Start cleanup goroutine
	spamProtection.startCleanup()

	log.Println("✅ Anti-spam protection initialized (2s delay)")
}

// startCleanup removes inactive users from memory
func (sp *CommandSpamProtection) startCleanup() {
	sp.cleanupTicker = time.NewTicker(5 * time.Minute)

	go func() {
		for range sp.cleanupTicker.C {
			sp.cleanup()
		}
	}()
}

func (sp *CommandSpamProtection) cleanup() {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	now := time.Now()
	inactiveThreshold := 10 * time.Minute

	// Clean up lastCommand
	for userID, lastTime := range sp.lastCommand {
		if now.Sub(lastTime) > inactiveThreshold {
			delete(sp.lastCommand, userID)
		}
	}

	// Clean up warning counts
	for userID, lastTime := range sp.lastCommand {
		if now.Sub(lastTime) > inactiveThreshold {
			delete(sp.warningCount, userID)
		}
	}

	// Clean up expired bans
	for userID, banTime := range sp.banUntil {
		if now.After(banTime) {
			delete(sp.banUntil, userID)
			log.Printf("🔓 User %d unbanned", userID)
		}
	}
}

// isCommand checks if message is a command
func isCommand(text string) bool {
	return strings.HasPrefix(text, "/")
}

// AntiSpamCommands enforces 2-second delay between commands
func AntiSpamCommands(next telebot.HandlerFunc) telebot.HandlerFunc {
	return func(c telebot.Context) error {
		userID := c.Sender().ID

		// Skip for bot owner
		ownerIDStr := os.Getenv("BOT_OWNER_ID")
		ownerID, _ := strconv.ParseInt(ownerIDStr, 10, 64)
		if userID == ownerID {
			return next(c)
		}

		// Check if user is banned
		if isBanned, until := spamProtection.isUserBanned(userID); isBanned {
			remaining := time.Until(until)
			return c.Send(fmt.Sprintf(
				"🚫 <b>Kamu dibanned sementara karena spam!</b>\n\n"+
					"Tunggu %d detik lagi.\n"+
					"Jangan spam command ya!",
				int(remaining.Seconds())+1,
			), telebot.ModeHTML)
		}

		// Check if this is a command
		text := c.Text()
		if !isCommand(text) {
			// Not a command, allow it
			return next(c)
		}

		// Check spam
		allowed, waitTime := spamProtection.allowCommand(userID)

		if !allowed {
			// User is spamming
			warnings := spamProtection.addWarning(userID)

			// Ban after 5 warnings
			if warnings >= 5 {
				spamProtection.banUser(userID, 5*time.Minute)
				log.Printf("🚫 User %d banned for 5 minutes (spam)", userID)

				return c.Send(
					"🚫 <b>BANNED!</b>\n\n"+
						"Kamu sudah terlalu banyak spam.\n"+
						"Akun diblokir selama 5 menit.\n\n"+
						"Jangan spam lagi ya!",
					telebot.ModeHTML,
				)
			}

			// Show warning
			return c.Send(fmt.Sprintf(
				"⏰ <b>Tunggu dulu!</b>\n\n"+
					"Kamu harus tunggu <b>%d detik</b> antara setiap command.\n"+
					"Peringatan: %d/5\n\n"+
					"⚠️ Kalau terus spam, akun akan diblokir sementara!",
				int(waitTime.Seconds())+1,
				warnings,
			), telebot.ModeHTML)
		}

		// Reset warning count on good behavior
		spamProtection.resetWarnings(userID)

		// Record this command
		spamProtection.recordCommand(userID)

		// Allow the command
		return next(c)
	}
}

// allowCommand checks if user can send a command
func (sp *CommandSpamProtection) allowCommand(userID int64) (bool, time.Duration) {
	sp.mu.RLock()
	lastTime, exists := sp.lastCommand[userID]
	sp.mu.RUnlock()

	if !exists {
		// First command, allow it
		return true, 0
	}

	elapsed := time.Since(lastTime)

	if elapsed < sp.commandDelay {
		// Too fast!
		waitTime := sp.commandDelay - elapsed
		return false, waitTime
	}

	// Enough time has passed
	return true, 0
}

// recordCommand records when user sent a command
func (sp *CommandSpamProtection) recordCommand(userID int64) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	sp.lastCommand[userID] = time.Now()
}

// addWarning increments warning count and returns new count
func (sp *CommandSpamProtection) addWarning(userID int64) int {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	sp.warningCount[userID]++
	return sp.warningCount[userID]
}

// resetWarnings resets warning count for good behavior
func (sp *CommandSpamProtection) resetWarnings(userID int64) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if count, exists := sp.warningCount[userID]; exists && count > 0 {
		sp.warningCount[userID] = 0
	}
}

// banUser temporarily bans a user
func (sp *CommandSpamProtection) banUser(userID int64, duration time.Duration) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	sp.banUntil[userID] = time.Now().Add(duration)
	sp.warningCount[userID] = 0 // Reset warnings
}

// isUserBanned checks if user is currently banned
func (sp *CommandSpamProtection) isUserBanned(userID int64) (bool, time.Time) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	banTime, exists := sp.banUntil[userID]
	if !exists {
		return false, time.Time{}
	}

	if time.Now().After(banTime) {
		// Ban expired
		return false, time.Time{}
	}

	return true, banTime
}

// GetUserStats returns spam protection stats for a user
func GetUserStats(userID int64) string {
	spamProtection.mu.RLock()
	defer spamProtection.mu.RUnlock()

	lastCmd, hasLastCmd := spamProtection.lastCommand[userID]
	warnings, _ := spamProtection.warningCount[userID]
	banTime, isBanned := spamProtection.banUntil[userID]

	var stats strings.Builder
	stats.WriteString("📊 <b>Anti-Spam Status</b>\n\n")

	if hasLastCmd {
		elapsed := time.Since(lastCmd)
		stats.WriteString(fmt.Sprintf("⏱ Last command: %d seconds ago\n", int(elapsed.Seconds())))

		if elapsed < 2*time.Second {
			canUseIn := 2*time.Second - elapsed
			stats.WriteString(fmt.Sprintf("🔒 Can use command in: %d seconds\n", int(canUseIn.Seconds())+1))
		} else {
			stats.WriteString("✅ Can use command now\n")
		}
	} else {
		stats.WriteString("✅ No recent commands\n")
	}

	stats.WriteString(fmt.Sprintf("⚠️ Warnings: %d/5\n", warnings))

	if isBanned && time.Now().Before(banTime) {
		remaining := time.Until(banTime)
		stats.WriteString(fmt.Sprintf("\n🚫 <b>BANNED</b> for %d more seconds\n", int(remaining.Seconds())+1))
	}

	return stats.String()
}

// ResetUserSpam resets spam protection for a user (admin command)
func ResetUserSpam(userID int64) {
	spamProtection.mu.Lock()
	defer spamProtection.mu.Unlock()

	delete(spamProtection.lastCommand, userID)
	delete(spamProtection.warningCount, userID)
	delete(spamProtection.banUntil, userID)

	log.Printf("🔄 Reset spam protection for user %d", userID)
}
