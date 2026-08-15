package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type FCMConfig struct {
	ProjectID   string
	AccessToken string
	HTTPClient  *http.Client
}

type FCMProvider struct {
	cfg FCMConfig
}

func NewFCMProvider(cfg FCMConfig) *FCMProvider {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &FCMProvider{cfg: cfg}
}

func (p *FCMProvider) Name() string { return "FCM" }

func (p *FCMProvider) Available() bool {
	return strings.TrimSpace(p.cfg.ProjectID) != "" && strings.TrimSpace(p.cfg.AccessToken) != ""
}

func (p *FCMProvider) Send(ctx context.Context, msg Message) (SendResult, error) {
	if !p.Available() {
		return SendResult{}, ErrProviderUnavailable
	}
	body := map[string]any{
		"message": map[string]any{
			"token": msg.Token,
			"notification": map[string]string{
				"title": msg.Title,
				"body":  msg.Title,
			},
			"data": map[string]string{
				"taskId":   msg.TaskID,
				"taskType": msg.TaskType,
			},
		},
	}
	raw, _ := json.Marshal(body)
	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", p.cfg.ProjectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return SendResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.cfg.HTTPClient.Do(req)
	if err != nil {
		return SendResult{}, ErrTransient
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return SendResult{}, ErrProviderUnavailable
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
		if strings.Contains(strings.ToLower(string(respBody)), "unregistered") ||
			strings.Contains(strings.ToLower(string(respBody)), "invalid") {
			return SendResult{}, ErrInvalidToken
		}
		return SendResult{}, fmt.Errorf("fcm bad request")
	}
	if resp.StatusCode >= 500 {
		return SendResult{}, ErrTransient
	}
	if resp.StatusCode >= 400 {
		return SendResult{}, fmt.Errorf("fcm status %d", resp.StatusCode)
	}
	var parsed struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(respBody, &parsed)
	return SendResult{ProviderMessageID: parsed.Name}, nil
}
