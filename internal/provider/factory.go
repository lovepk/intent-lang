package provider

import (
	"fmt"
	"os"
	"strings"
)

// DeepSeek defaults. DeepSeek is just an OpenAI-compatible endpoint, so it is
// implemented on top of OpenAICompat.
const (
	deepseekBaseURL = "https://api.deepseek.com"
	deepseekModel   = "deepseek-chat"
)

// FromEnv builds a provider by name, reading credentials and endpoint from
// environment variables. key selects a credential slot (typically "A" or "B")
// so two instances of the same provider can play LLM-A / LLM-B.
//
// For provider "p" (upper-cased to P) and key "K", values are read in order:
//
//	API key:  INTENT_P_API_KEY_K > INTENT_P_API_KEY > P_API_KEY_K > P_API_KEY
//	Base URL: INTENT_P_BASE_URL   > P_BASE_URL
//	Model:    INTENT_P_MODEL      > P_MODEL
//
// DeepSeek keeps its existing variable names (DEEPSEEK_API_KEY_A,
// DEEPSEEK_BASE_URL, DEEPSEEK_MODEL) and defaults BaseURL/Model when unset.
//
// The API key may be empty when BaseURL is explicitly set (local
// OpenAI-compatible servers such as Ollama usually need no key).
func FromEnv(name, key string) (Provider, error) {
	p, prefix, k := resolveName(name, key)
	apiKey := firstEnv(keyCandidates(prefix, k)...)
	baseURL := firstEnv(baseCandidates(prefix)...)
	baseExplicit := baseURL != ""
	model := firstEnv(modelCandidates(prefix)...)

	if p == "deepseek" {
		if baseURL == "" {
			baseURL = deepseekBaseURL
		}
		if model == "" {
			model = deepseekModel
		}
	}

	if baseURL == "" {
		return nil, fmt.Errorf("provider %q: 缺少 base URL（设置 %s_BASE_URL）", p, prefix)
	}
	if model == "" {
		return nil, fmt.Errorf("provider %q: 缺少 model（设置 %s_MODEL）", p, prefix)
	}
	if apiKey == "" && !baseExplicit {
		return nil, fmt.Errorf("provider %q: 缺少 API key（设置 %s_API_KEY_%s）", p, prefix, k)
	}
	return NewOpenAICompat(OpenAIConfig{Name: p, APIKey: apiKey, BaseURL: baseURL, Model: model}), nil
}

// Configured reports whether the provider has any explicit configuration in
// the environment (an API key, or a base URL for non-DeepSeek providers). Used
// to decide whether the MCP server should expose the model-driven tools.
func Configured(name, key string) bool {
	p, prefix, k := resolveName(name, key)
	if firstEnv(keyCandidates(prefix, k)...) != "" {
		return true
	}
	return p != "deepseek" && firstEnv(baseCandidates(prefix)...) != ""
}

func resolveName(name, key string) (p, prefix, k string) {
	p = strings.ToLower(strings.TrimSpace(name))
	if p == "" {
		p = "deepseek"
	}
	prefix = strings.ToUpper(strings.ReplaceAll(p, "-", "_"))
	k = strings.ToUpper(strings.TrimSpace(key))
	if k == "" {
		k = "A"
	}
	return p, prefix, k
}

func keyCandidates(prefix, k string) []string {
	return []string{
		"INTENT_" + prefix + "_API_KEY_" + k,
		"INTENT_" + prefix + "_API_KEY",
		prefix + "_API_KEY_" + k,
		prefix + "_API_KEY",
	}
}

func baseCandidates(prefix string) []string {
	return []string{"INTENT_" + prefix + "_BASE_URL", prefix + "_BASE_URL"}
}

func modelCandidates(prefix string) []string {
	return []string{"INTENT_" + prefix + "_MODEL", prefix + "_MODEL"}
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}
