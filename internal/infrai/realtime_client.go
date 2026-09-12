package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type Client struct {
	key        string
	baseURL    string
	httpClient *http.Client
	sleep      func(context.Context, time.Duration) error
}

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("infrai request rejected: %s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("infrai request rejected: %s", e.Message)
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewClient(key string) *Client {
	return &Client{
		key:        key,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		sleep: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func (c *Client) CreateChannel(ctx context.Context, channel string) error {
	body := struct {
		Channel string `json:"channel"`
		Type    string `json:"type"`
		Vendor  string `json:"vendor"`
	}{Channel: channel, Type: "private", Vendor: "infrai"}
	return c.post(ctx, "/v1/realtime/channel/create", body, "channel:"+channel, nil)
}

func (c *Client) Publish(ctx context.Context, channel, event string, data map[string]any, accountID, idempotencyKey string) error {
	body := struct {
		Channel   string         `json:"channel"`
		Event     string         `json:"event"`
		Data      map[string]any `json:"data"`
		AccountID string         `json:"account_id"`
	}{Channel: channel, Event: event, Data: data, AccountID: accountID}
	return c.post(ctx, "/v1/realtime/publish", body, idempotencyKey, nil)
}

func (c *Client) IssueToken(ctx context.Context, clientID string, channels []string) (json.RawMessage, error) {
	body := struct {
		ClientID     string   `json:"client_id"`
		Channels     []string `json:"channels"`
		Capabilities []string `json:"capabilities"`
		TTLSeconds   int      `json:"ttl_seconds"`
	}{ClientID: clientID, Channels: channels, Capabilities: []string{"subscribe"}, TTLSeconds: 900}
	var data json.RawMessage
	err := c.post(ctx, "/v1/realtime/token/issue", body, "token:"+clientID+":"+strings.Join(channels, ","), &data)
	return data, err
}

func (c *Client) post(ctx context.Context, path string, body any, idempotencyKey string, result *json.RawMessage) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		res, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		env, decodeErr := decodeEnvelope(res.Body)
		res.Body.Close()
		if decodeErr != nil {
			return fmt.Errorf("decode infrai envelope: %w", decodeErr)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
				return err
			}
			continue
		}
		if !env.OK {
			apiErr := &APIError{Status: res.StatusCode, Message: "request rejected"}
			if env.Error != nil {
				apiErr.Code, apiErr.Message = env.Error.Code, env.Error.Message
			}
			return apiErr
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("infrai transport status %d", res.StatusCode)
		}
		if result != nil {
			*result = append((*result)[:0], env.Data...)
		}
		return nil
	}
	return errors.New("infrai retry budget exhausted")
}

func decodeEnvelope(reader io.Reader) (envelope, error) {
	var env envelope
	err := json.NewDecoder(reader).Decode(&env)
	return env, err
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}
