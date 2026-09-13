package provider

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

// OpenAIConfig configures a provider that speaks the OpenAI-compatible
// chat-completions protocol. Any vendor exposing that protocol (DeepSeek,
// OpenAI, Qwen, a local server, ...) can be used by setting BaseURL/Model.
type OpenAIConfig struct {
	Name    string
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// OpenAICompat is the generic transport for OpenAI-compatible endpoints. It
// only transports the model's draft: validity and self-consistency are the
// Agent's job (L1 deterministic + L2 recorder). It never rejects, retries or
// rewrites output — that would be "interference", which the record-only model
// forbids.
type OpenAICompat struct {
	name    string
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

func NewOpenAICompat(cfg OpenAIConfig) *OpenAICompat {
	if cfg.Name == "" {
		cfg.Name = "openai-compatible"
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	return &OpenAICompat{
		name:    cfg.Name,
		apiKey:  cfg.APIKey,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		model:   cfg.Model,
		client:  &http.Client{Timeout: timeout},
	}
}

func (o *OpenAICompat) Name() string { return o.name }

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

func (o *OpenAICompat) Complete(ctx context.Context, req Request) (Response, error) {
	if o.apiKey == "" {
		return Response{}, fmt.Errorf("%s: empty api key", o.name)
	}

	userText := req.User
	if strings.TrimSpace(req.Archive) != "" {
		userText = fmt.Sprintf("当前意图档案（可能为空）：\n```\n%s\n```\n\n用户本次需求：\n%s", req.Archive, req.User)
	}

	msg := chatRequest{
		Model: o.model,
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

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("%s: request: %w", o.name, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("%s: read body: %w", o.name, err)
	}
	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("%s: status %d: %s", o.name, resp.StatusCode, truncate(string(raw), 500))
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return Response{}, fmt.Errorf("%s: decode: %w", o.name, err)
	}
	if len(cr.Choices) == 0 {
		return Response{}, fmt.Errorf("%s: no choices returned", o.name)
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
		return Response{}, fmt.Errorf("%s: model did not return valid JSON: %w\nraw: %s", o.name, err, truncate(content, 800))
	}

	if req.Mode == ModeLint {
		var lint struct {
			Findings []Finding `json:"findings"`
		}
		if err := json.Unmarshal([]byte(content), &lint); err != nil {
			return Response{}, fmt.Errorf("%s: lint output invalid: %w", o.name, err)
		}
		return Response{Findings: lint.Findings}, nil
	}

	return Response{Reply: out.Reply, IntentUpdate: out.IntentUpdate}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}

var _ Provider = (*OpenAICompat)(nil)
