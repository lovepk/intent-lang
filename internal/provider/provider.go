package provider

import (
	"context"
	"time"
)

type Request struct {
	System  string
	Archive string
	User    string
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
