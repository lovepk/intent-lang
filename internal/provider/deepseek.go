package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type DeepSeek struct {
	name    string
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

type DeepSeekConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Name    string
}

func NewDeepSeek(cfg DeepSeekConfig) *DeepSeek {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.deepseek.com"
	}
	if cfg.Model == "" {
		cfg.Model = "deepseek-chat"
	}
	if cfg.Name == "" {
		cfg.Name = "deepseek"
	}
	return &DeepSeek{
		name:    cfg.Name,
		apiKey:  cfg.APIKey,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		model:   cfg.Model,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func NewDeepSeekFromEnv(keyEnv string) *DeepSeek {
	return NewDeepSeek(DeepSeekConfig{
		APIKey:  os.Getenv(keyEnv),
		BaseURL: os.Getenv("DEEPSEEK_BASE_URL"),
		Model:   os.Getenv("DEEPSEEK_MODEL"),
		Name:    keyEnv,
	})
}

func (d *DeepSeek) Name() string { return d.name }

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (d *DeepSeek) Complete(ctx context.Context, req Request) (Response, error) {
	if d.apiKey == "" {
		return Response{}, errors.New("deepseek: empty api key")
	}

	userText := req.User
	if strings.TrimSpace(req.Archive) != "" {
		userText = fmt.Sprintf("当前意图档案（可能为空）：\n```\n%s\n```\n\n用户本次需求：\n%s", req.Archive, req.User)
	}

	msg := chatRequest{
		Model: d.model,
		Messages: []chatMessage{
			{Role: "system", Content: req.System},
			{Role: "user", Content: userText},
		},
	}
	if req.Mode != ModeRepro && req.Mode != ModeNormalize {
		msg.ResponseFormat = &responseFormat{Type: "json_object"}
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return Response{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+d.apiKey)

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("deepseek: request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("deepseek: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("deepseek: status %d: %s", resp.StatusCode, truncate(string(raw), 500))
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return Response{}, fmt.Errorf("deepseek: decode: %w", err)
	}
	if len(cr.Choices) == 0 {
		return Response{}, errors.New("deepseek: no choices returned")
	}
	content := cr.Choices[0].Message.Content

	if req.Mode == ModeRepro {
		return Response{Reply: StripFence(content)}, nil
	}
	if req.Mode == ModeNormalize {
		return Response{Reply: StripFence(strings.TrimSpace(content))}, nil
	}

	var out struct {
		IntentUpdate string `json:"intent_update"`
		Reply        string `json:"reply"`
	}
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return Response{}, fmt.Errorf("deepseek: model did not return valid JSON: %w\nraw: %s", err, truncate(content, 800))
	}

	if req.Mode == ModeLint {
		var lint struct {
			Findings []Finding `json:"findings"`
		}
		if err := json.Unmarshal([]byte(content), &lint); err != nil {
			return Response{}, fmt.Errorf("deepseek: lint output invalid: %w", err)
		}
		return Response{Findings: lint.Findings}, nil
	}

	// No IL validation/normalization here: the provider only transports the
	// model's draft. Validity and self-consistency are the job of the layered
	// checks in the agent (L1 deterministic, L2 recorder). Rejecting here would
	// be "interference", which the record-only model forbids.
	return Response{Reply: out.Reply, IntentUpdate: out.IntentUpdate}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
