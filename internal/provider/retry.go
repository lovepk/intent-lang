package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Retry is a decorator that re-issues a failing write-mode request up to
// attempts-1 additional times, appending a corrective instruction when the
// model returned something that failed IL validation.
type Retry struct {
	inner    Provider
	attempts int
}

func NewRetry(inner Provider, attempts int) *Retry {
	if attempts < 1 {
		attempts = 1
	}
	return &Retry{inner: inner, attempts: attempts}
}

func (r *Retry) Name() string { return r.inner.Name() }

func (r *Retry) Complete(ctx context.Context, req Request) (Response, error) {
	var lastErr error
	for i := 0; i < r.attempts; i++ {
		resp, err := r.inner.Complete(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if req.Mode == ModeRepro || i == r.attempts-1 {
			break
		}
		req = amend(req, err)
	}
	if lastErr == nil {
		return Response{}, errors.New("retry: no attempt made")
	}
	return Response{}, lastErr
}

func amend(req Request, err error) Request {
	note := "你上次的输出不合法，请修正后重试。"
	if msg := err.Error(); strings.Contains(msg, "JSON") {
		note = "你上次没有返回合法的 JSON。请严格输出单个 JSON 对象：{\"reply\": \"...\", \"intent_update\": \"完整档案文本\"}，不要加任何其他文字。"
	} else if strings.Contains(msg, "fails IL validation") || strings.Contains(msg, "not parseable IL") {
		note = "你上次的 intent_update 不满足 IL 规范（可能是空段、缺头、编号重复、OPEN 缺 default、段体重复等）。请用档案语言重写整份合法档案。错误：" + err.Error()
	}
	req.User = fmt.Sprintf("%s\n\n%s", req.User, note)
	return req
}
