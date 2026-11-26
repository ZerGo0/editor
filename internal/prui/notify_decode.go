package prui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const subjectPullRequest = "PullRequest"

func decodeNotifications(data []byte) ([]Notification, error) {
	var raw []struct {
		ID        string    `json:"id"`
		Unread    bool      `json:"unread"`
		Reason    string    `json:"reason"`
		UpdatedAt time.Time `json:"updated_at"`
		Subject   struct {
			Title string `json:"title"`
			URL   string `json:"url"`
			Type  string `json:"type"`
		} `json:"subject"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("prui: decode notifications: %w", err)
	}
	var items []Notification
	for _, n := range raw {
		if n.ID == "" || n.Subject.Type != subjectPullRequest {
			continue
		}
		number, err := strconv.Atoi(n.Subject.URL[strings.LastIndex(n.Subject.URL, "/")+1:])
		if err != nil || number <= 0 {
			continue
		}
		items = append(items, Notification{ID: n.ID, Title: n.Subject.Title, Number: number, Reason: n.Reason, Unread: n.Unread, Updated: n.UpdatedAt})
	}
	return items, nil
}
