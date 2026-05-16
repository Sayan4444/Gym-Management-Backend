package jobs

import (
	"fmt"
	"log"
	"time"

	"gym-saas/database"
	"gym-saas/models"
	"gym-saas/utils"

	"github.com/robfig/cron/v3"
)

// StartSubscriptionCron initializes and starts the daily background jobs for subscriptions.
func StartSubscriptionCron() {
	// Create a new cron instance with default parser (minutes, hours, dom, month, dow)
	c := cron.New()

	// Schedule the job to run every day at midnight.
	// We use the "@daily" descriptor which is eq to "0 0 * * *"
	_, err := c.AddFunc("@daily", func() {
		log.Println("[CRON] Running daily subscription notifications")
		notifyExpiredSubscriptions()
		notifyExpiringSoonSubscriptions()
	})

	if err != nil {
		log.Fatalf("[CRON ERROR] Failed to schedule subscription job: %v", err)
	}

	c.Start()
	log.Println("[CRON] Subscription jobs scheduled successfully")
}

// notifyExpiredSubscriptions sends email notifications for subscriptions that
// expired within the last 24 hours. No status mutation is needed because
// CurrentStatus() dynamically computes "Expired" from the EndDate.
func notifyExpiredSubscriptions() {
	var expiredSubs []models.Subscription

	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)

	// Find subscriptions that ended in the last 24 hours and are not manually overridden
	if err := database.DB.
		Where("(status NOT IN (?) OR status = '') AND end_date > ? AND end_date <= ?",
			[]string{"Paused", "Cancelled"}, yesterday, now).
		Find(&expiredSubs).Error; err != nil {
		log.Printf("[CRON ERROR] Failed to fetch recently expired subscriptions: %v\n", err)
		return
	}

	for _, sub := range expiredSubs {
		var user models.User
		if err := database.DB.First(&user, sub.UserID).Error; err != nil {
			log.Printf("[CRON ERROR] User not found for expired subscription %d", sub.ID)
			continue
		}

		// Send expiration email
		subject := "Your Gym Package has Expired"
		body := fmt.Sprintf("Dear %s,\n\nThis is a notification that your gym package has expired on %s.\n\nPlease renew your plan to continue accessing the gym facilities.\n\nBest Regards,\nGym Management Team", user.Name, sub.EndDate.Format("Jan 02, 2006"))
		go utils.SendEmail(user.Email, subject, body)
	}

	if len(expiredSubs) > 0 {
		log.Printf("[CRON] Sent %d expiration notifications\n", len(expiredSubs))
	}
}

// notifyExpiringSoonSubscriptions sends reminder emails for subscriptions
// expiring within the next 24 hours.
func notifyExpiringSoonSubscriptions() {
	var expiringSubs []models.Subscription

	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)

	// Fetch active subscriptions expiring strictly within the next 24 hours.
	if err := database.DB.
		Where("(status NOT IN (?) OR status = '') AND start_date <= ? AND end_date > ? AND end_date <= ?",
			[]string{"Paused", "Cancelled"}, now, now, tomorrow).
		Find(&expiringSubs).Error; err != nil {
		log.Printf("[CRON ERROR] Failed to fetch expiring soon subscriptions: %v\n", err)
		return
	}

	for _, sub := range expiringSubs {
		var user models.User
		if err := database.DB.First(&user, sub.UserID).Error; err != nil {
			log.Printf("[CRON ERROR] User not found for expiring subscription %d", sub.ID)
			continue
		}

		// Send reminder email
		subject := "Reminder: Your Gym Package Expires Tomorrow"
		body := fmt.Sprintf("Dear %s,\n\nThis is a friendly reminder that your gym package is set to expire tomorrow, %s.\n\nPlease renew your plan soon to avoid any interruption in your access.\n\nBest Regards,\nGym Management Team", user.Name, sub.EndDate.Format("Jan 02, 2006"))
		go utils.SendEmail(user.Email, subject, body)
	}

	if len(expiringSubs) > 0 {
		log.Printf("[CRON] Successfully sent %d expiration reminders\n", len(expiringSubs))
	}
}
