package relay

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// NtfyMessage represents a message received from the ntfy JSON stream.
type NtfyMessage struct {
	ID      string `json:"id"`
	Time    int64  `json:"time"`
	Event   string `json:"event"`
	Topic   string `json:"topic"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

// SpeakRequest represents the request body for POST /v1/speak.
type SpeakRequest struct {
	Text      string `json:"text"`
	Interrupt bool   `json:"interrupt,omitempty"`
	TTLMS     int    `json:"ttl_ms,omitempty"`
	DedupeKey string `json:"dedupe_key,omitempty"`
}

// Client is the ntfy relay client that subscribes to ntfy topics
// and forwards messages to the Discorgeous API.
type Client struct {
	cfg        *Config
	logger     *slog.Logger
	httpClient *http.Client
	dedupeMap  map[string]time.Time
	dedupeMu   sync.Mutex
}

// NewClient creates a new relay client.
func NewClient(cfg *Config, logger *slog.Logger) *Client {
	return &Client{
		cfg:    cfg,
		logger: logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		dedupeMap: make(map[string]time.Time),
	}
}

// Run starts the relay client, subscribing to all configured topics.
// It blocks until the context is cancelled.
func (c *Client) Run(ctx context.Context) error {
	var wg sync.WaitGroup

	for _, topic := range c.cfg.NtfyTopics {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()
			c.subscribeLoop(ctx, t)
		}(topic)
	}

	// Start dedupe cleanup goroutine if dedupe is enabled
	if c.cfg.DedupeWindow > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.dedupeCleanupLoop(ctx)
		}()
	}

	wg.Wait()
	return nil
}

// subscribeLoop subscribes to a single topic and reconnects on errors.
func (c *Client) subscribeLoop(ctx context.Context, topic string) {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		c.logger.Info("subscribing to ntfy topic", "topic", topic, "server", c.cfg.NtfyServer)

		err := c.subscribe(ctx, topic)
		if err != nil {
			if ctx.Err() != nil {
				// Context was cancelled, exit gracefully
				return
			}
			c.logger.Warn("subscription error, reconnecting", "topic", topic, "error", err, "backoff", backoff)
		}

		// Wait before reconnecting
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		// Exponential backoff
		backoff = backoff * 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// subscribe connects to the ntfy JSON stream for a topic and processes messages.
func (c *Client) subscribe(ctx context.Context, topic string) error {
	url := fmt.Sprintf("%s/%s/json", strings.TrimSuffix(c.cfg.NtfyServer, "/"), topic)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Use a client without timeout for streaming
	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	c.logger.Info("connected to ntfy stream", "topic", topic)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg NtfyMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			c.logger.Warn("failed to parse ntfy message", "error", err, "line", string(line))
			continue
		}

		// Skip non-message events (keepalive, open, etc.)
		if msg.Event != "message" {
			c.logger.Debug("skipping non-message event", "event", msg.Event, "topic", topic)
			continue
		}

		c.handleMessage(msg)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

// handleMessage processes a single ntfy message and forwards it to Discorgeous.
func (c *Client) handleMessage(msg NtfyMessage) {
	c.logger.Debug("received ntfy message",
		"id", msg.ID,
		"topic", msg.Topic,
		"title", msg.Title,
		"message", msg.Message,
	)

	// Build the text to speak
	text := c.FormatText(msg.Title, msg.Message)
	if text == "" {
		c.logger.Debug("skipping empty message", "id", msg.ID)
		return
	}

	// Generate dedupe key if dedupe window is enabled
	var dedupeKey string
	if c.cfg.DedupeWindow > 0 {
		dedupeKey = c.generateDedupeKey(text)
		if c.isDuplicate(dedupeKey) {
			c.logger.Debug("skipping duplicate message", "id", msg.ID, "dedupe_key", dedupeKey)
			return
		}
		c.recordDedupeKey(dedupeKey)
	}

	// Forward to Discorgeous
	if err := c.forwardToDiscorgeous(text, dedupeKey); err != nil {
		c.logger.Error("failed to forward message to Discorgeous",
			"error", err,
			"ntfy_id", msg.ID,
			"text_length", len(text),
		)
		return
	}

	c.logger.Info("forwarded message to Discorgeous",
		"ntfy_id", msg.ID,
		"topic", msg.Topic,
		"text_length", len(text),
		"interrupt", c.cfg.Interrupt,
	)
}

// FormatText combines title and message with optional prefix and enforces max length.
func (c *Client) FormatText(title, message string) string {
	var parts []string

	if c.cfg.Prefix != "" {
		parts = append(parts, c.cfg.Prefix)
	}

	if title != "" {
		parts = append(parts, title)
	}

	if message != "" {
		parts = append(parts, message)
	}

	text := strings.Join(parts, ": ")

	// Enforce max length
	if len(text) > c.cfg.MaxTextLength {
		text = text[:c.cfg.MaxTextLength]
	}

	return text
}

// forwardToDiscorgeous sends the text to the Discorgeous /v1/speak API.
func (c *Client) forwardToDiscorgeous(text, dedupeKey string) error {
	url := fmt.Sprintf("%s/v1/speak", strings.TrimSuffix(c.cfg.DiscorgeousAPIURL, "/"))

	speakReq := SpeakRequest{
		Text:      text,
		Interrupt: c.cfg.Interrupt,
		DedupeKey: dedupeKey,
	}

	body, err := json.Marshal(speakReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.cfg.DiscorgeousBearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.DiscorgeousBearerToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// generateDedupeKey creates a hash-based dedupe key from the text.
func (c *Client) generateDedupeKey(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:8])
}

// isDuplicate checks if a dedupe key has been seen within the dedupe window.
func (c *Client) isDuplicate(key string) bool {
	c.dedupeMu.Lock()
	defer c.dedupeMu.Unlock()

	if seenAt, ok := c.dedupeMap[key]; ok {
		if time.Since(seenAt) < c.cfg.DedupeWindow {
			return true
		}
	}
	return false
}

// recordDedupeKey records a dedupe key with the current timestamp.
func (c *Client) recordDedupeKey(key string) {
	c.dedupeMu.Lock()
	defer c.dedupeMu.Unlock()
	c.dedupeMap[key] = time.Now()
}

// dedupeCleanupLoop periodically removes expired dedupe keys.
func (c *Client) dedupeCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(c.cfg.DedupeWindow)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.cleanupDedupeMap()
		}
	}
}

// cleanupDedupeMap removes dedupe keys older than the dedupe window.
func (c *Client) cleanupDedupeMap() {
	c.dedupeMu.Lock()
	defer c.dedupeMu.Unlock()

	now := time.Now()
	for key, seenAt := range c.dedupeMap {
		if now.Sub(seenAt) >= c.cfg.DedupeWindow {
			delete(c.dedupeMap, key)
		}
	}
}
