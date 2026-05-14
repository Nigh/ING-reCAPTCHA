package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	modelName  string
	httpClient *http.Client
	enabled    bool
}

func NewClient(baseURL, apiKey, modelName string, timeoutSeconds int) *Client {
	if baseURL == "" || apiKey == "" || modelName == "" {
		return &Client{enabled: false}
	}

	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiKey:    apiKey,
		modelName: modelName,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
		enabled: true,
	}
}

func (c *Client) IsEnabled() bool {
	return c.enabled
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) HealthCheck(ctx context.Context) error {
	req := chatRequest{
		Model:       c.modelName,
		Messages:    []chatMessage{{Role: "user", Content: "ping"}},
		Temperature: 0,
		MaxTokens:   1,
	}

	_, err := c.doRequest(ctx, req)
	return err
}

func (c *Client) AssessRisk(ctx context.Context, userInfo *UserInfo) (int, error) {
	if !c.enabled {
		return 0, fmt.Errorf("AI client is not enabled")
	}

	systemMsg := buildSystemPrompt()
	userMsg := buildUserPrompt(userInfo)

	req := chatRequest{
		Model:       c.modelName,
		Messages:    []chatMessage{{Role: "system", Content: systemMsg}, {Role: "user", Content: userMsg}},
		Temperature: 0.3,
		MaxTokens:   100,
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("AI request failed: %w", err)
	}

	questionCount, err := parseQuestionCount(resp)
	if err != nil {
		return 0, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return questionCount, nil
}

func (c *Client) doRequest(ctx context.Context, req chatRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}

var jsonBlockRe = regexp.MustCompile("(?s)```(?:json)?\\s*(.+?)```")
var numberRe = regexp.MustCompile(`"question_count"\s*:\s*(\d+)`)

func parseQuestionCount(content string) (int, error) {
	extracted := content

	if matches := jsonBlockRe.FindStringSubmatch(content); len(matches) > 1 {
		extracted = matches[1]
	}

	var result struct {
		QuestionCount int `json:"question_count"`
	}

	if err := json.Unmarshal([]byte(strings.TrimSpace(extracted)), &result); err == nil {
		return clampQuestionCount(result.QuestionCount), nil
	}

	if matches := numberRe.FindStringSubmatch(content); len(matches) > 1 {
		var n int
		if _, err := fmt.Sscanf(matches[1], "%d", &n); err == nil {
			return clampQuestionCount(n), nil
		}
	}

	log.Printf("[WARN] AI response did not contain parseable question_count, raw: %s", content)
	return 0, fmt.Errorf("could not extract question_count from response: %s", content)
}

func clampQuestionCount(n int) int {
	if n < 3 {
		return 3
	}
	if n > 7 {
		return 7
	}
	return n
}
