package provider

import (
	"context"
)

type Mode string

const (
	ModeWrite     Mode = "write"
	ModeRepro     Mode = "repro"
	ModeLint      Mode = "lint"
	ModeNormalize Mode = "normalize"
)

type Request struct {
	System  string
	Archive string
	User    string
	Mode    Mode
}

// Finding is a semantic lint diagnostic produced by an LLM reviewer.
type Finding struct {
	Severity string `json:"severity"`
	Section  string `json:"section"`
	ID       string `json:"id"`
	Msg      string `json:"msg"`
	Fix      string `json:"fix"`
}

type Response struct {
	Reply        string
	IntentUpdate string
	Findings     []Finding
}

type Provider interface {
	Complete(ctx context.Context, req Request) (Response, error)
	Name() string
}
