package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type LINE struct {
	token, recipient string
	enabled          bool
	client           *http.Client
	endpoint         string
}

func NewLINE(enabled bool, token, recipient string) *LINE {
	return &LINE{enabled: enabled, token: token, recipient: recipient, client: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, endpoint: "https://api.line.me/v2/bot/message/push"}
}
func (l *LINE) Send(ctx context.Context, e Event) error {
	if !l.enabled {
		return ErrDisabled
	}
	if l.token == "" || l.recipient == "" {
		return errors.New("LINE is not configured")
	}
	// No customer name, attachment, evidence or secret appears in the message.
	body, _ := json.Marshal(map[string]any{"to": l.recipient, "messages": []map[string]string{{"type": "text", "text": fmt.Sprintf("New manufacturing request #%d (%s). Open the shop system to review.", e.RequestNumber, e.RequestID)}}})
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, l.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+l.token)
	r.Header.Set("X-Line-Retry-Key", e.ID)
	response, err := l.client.Do(r)
	if err != nil {
		return errors.New("LINE transport failed")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode == 200 || (response.StatusCode == 409 && response.Header.Get("X-Line-Accepted-Request-Id") != "") {
		return nil
	}
	return fmt.Errorf("LINE returned HTTP %d", response.StatusCode)
}
