package provider

import (
	"context"
	"time"
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

type Meta struct {
	CreatedAt time.Time
	Model     string
}
