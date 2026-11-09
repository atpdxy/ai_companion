package domain

import (
	"time"
	_ "time/tzdata"
)

type NotificationPreferences struct {
	OptIn      bool   `json:"opt_in"`
	NoContact  bool   `json:"no_contact"`
	QuietStart int    `json:"quiet_start"`
	QuietEnd   int    `json:"quiet_end"`
	DailyLimit int    `json:"daily_limit"`
	Timezone   string `json:"timezone"`
}

func DefaultNotificationPreferences() NotificationPreferences {
	return NotificationPreferences{QuietStart: 22, QuietEnd: 8, DailyLimit: 3, Timezone: "Asia/Shanghai"}
}
func (p NotificationPreferences) Valid() bool {
	_, err := time.LoadLocation(p.Timezone)
	return err == nil && p.QuietStart >= 0 && p.QuietStart <= 23 && p.QuietEnd >= 0 && p.QuietEnd <= 23 && p.DailyLimit >= 1 && p.DailyLimit <= 10 && (!p.OptIn || !p.NoContact)
}
func (p NotificationPreferences) Eligible(now time.Time, delivered int) bool {
	if !p.OptIn || p.NoContact || delivered >= p.DailyLimit {
		return false
	}
	location, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return false
	}
	hour := now.In(location).Hour()
	if p.QuietStart == p.QuietEnd {
		return true
	}
	if p.QuietStart < p.QuietEnd {
		return hour < p.QuietStart || hour >= p.QuietEnd
	}
	return hour < p.QuietStart && hour >= p.QuietEnd
}

type ScheduledEvent struct {
	ID         string    `json:"id"`
	Content    string    `json:"content"`
	DueAt      time.Time `json:"due_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Recurrence string    `json:"recurrence"`
	Status     string    `json:"status"`
	ClientKey  string    `json:"client_key"`
}
type Notification struct {
	ID          string    `json:"id"`
	EventID     string    `json:"event_id"`
	Content     string    `json:"content"`
	DeliveredAt time.Time `json:"delivered_at"`
}
