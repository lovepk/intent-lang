package provider

import (
	"context"
)

type Mode string

const (
	ModeWrite Mode = "write"
	ModeRepro Mode = "repro"
)

type Request struct {
	System  string
	Archive string
	User    string
	Mode    Mode
}

type Response struct {
	Reply        string
	IntentUpdate string
}

type Provider interface {
	Complete(ctx context.Context, req Request) (Response, error)
	Name() string
}
