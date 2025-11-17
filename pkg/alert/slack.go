package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type SlackMessage struct {
	Text string `json:"text"`
}

func Post(webhook string, text string) error {
	payload, _ := json.Marshal(SlackMessage{Text: text})
	client := &http.Client{ Timeout: 10 * time.Second }
	resp, err := client.Post(webhook, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack webhook returned status %s", resp.Status)
	}
	return nil
}
