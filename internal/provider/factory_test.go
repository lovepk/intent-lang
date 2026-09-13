package provider

import "testing"

// clearDeepSeek blanks the DeepSeek env vars so tests are independent of any
// ambient configuration. t.Setenv("", ...) makes firstEnv treat them as unset.
func clearDeepSeek(t *testing.T) {
	for _, k := range []string{
		"DEEPSEEK_API_KEY_A", "DEEPSEEK_API_KEY_B", "DEEPSEEK_API_KEY",
		"INTENT_DEEPSEEK_API_KEY_A", "INTENT_DEEPSEEK_API_KEY",
		"DEEPSEEK_BASE_URL", "INTENT_DEEPSEEK_BASE_URL",
		"DEEPSEEK_MODEL", "INTENT_DEEPSEEK_MODEL",
	} {
		t.Setenv(k, "")
	}
}

func TestFromEnvDeepSeekDefaults(t *testing.T) {
	clearDeepSeek(t)
	t.Setenv("DEEPSEEK_API_KEY_A", "sk-a")
	p, err := FromEnv("deepseek", "A")
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	oc := p.(*OpenAICompat)
	if oc.name != "deepseek" || oc.apiKey != "sk-a" {
		t.Errorf("name/key = %q/%q", oc.name, oc.apiKey)
	}
	if oc.baseURL != deepseekBaseURL || oc.model != deepseekModel {
		t.Errorf("baseURL/model = %q/%q", oc.baseURL, oc.model)
	}
}

func TestFromEnvDeepSeekKeyB(t *testing.T) {
	clearDeepSeek(t)
	t.Setenv("DEEPSEEK_API_KEY_A", "sk-a")
	t.Setenv("DEEPSEEK_API_KEY_B", "sk-b")
	p, err := FromEnv("deepseek", "B")
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if got := p.(*OpenAICompat).apiKey; got != "sk-b" {
		t.Errorf("apiKey = %q, want sk-b", got)
	}
}

func TestFromEnvGenericProvider(t *testing.T) {
	t.Setenv("OPENAI_API_KEY_A", "sk-openai")
	t.Setenv("OPENAI_BASE_URL", "https://api.openai.com/v1")
	t.Setenv("OPENAI_MODEL", "gpt-4o-mini")
	p, err := FromEnv("openai", "A")
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	oc := p.(*OpenAICompat)
	if oc.name != "openai" || oc.apiKey != "sk-openai" {
		t.Errorf("name/key = %q/%q", oc.name, oc.apiKey)
	}
	if oc.baseURL != "https://api.openai.com/v1" || oc.model != "gpt-4o-mini" {
		t.Errorf("baseURL/model = %q/%q", oc.baseURL, oc.model)
	}
}

func TestFromEnvIntentPrefixPrecedence(t *testing.T) {
	t.Setenv("OPENAI_API_KEY_A", "legacy")
	t.Setenv("INTENT_OPENAI_API_KEY_A", "preferred")
	t.Setenv("OPENAI_BASE_URL", "https://api.openai.com/v1")
	t.Setenv("OPENAI_MODEL", "gpt-4o-mini")
	p, err := FromEnv("openai", "A")
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if got := p.(*OpenAICompat).apiKey; got != "preferred" {
		t.Errorf("apiKey = %q, want preferred", got)
	}
}

func TestFromEnvDeepSeekRequiresKey(t *testing.T) {
	clearDeepSeek(t)
	if _, err := FromEnv("deepseek", "A"); err == nil {
		t.Fatal("expected error for missing DeepSeek API key")
	}
}

func TestFromEnvLocalProviderKeyOptional(t *testing.T) {
	t.Setenv("OLLAMA_BASE_URL", "http://localhost:11434/v1")
	t.Setenv("OLLAMA_MODEL", "qwen2.5:7b")
	p, err := FromEnv("ollama", "A")
	if err != nil {
		t.Fatalf("local provider without key should work: %v", err)
	}
	oc := p.(*OpenAICompat)
	if oc.apiKey != "" || oc.baseURL != "http://localhost:11434/v1" {
		t.Errorf("local provider = %+v", oc)
	}
}

func TestFromEnvGenericRequiresEndpoint(t *testing.T) {
	t.Setenv("OPENAI_API_KEY_A", "sk")
	if _, err := FromEnv("openai", "A"); err == nil {
		t.Fatal("expected error for missing base URL/model")
	}
}

func TestConfigured(t *testing.T) {
	clearDeepSeek(t)
	if Configured("deepseek", "A") {
		t.Error("deepseek with no env should not be configured")
	}
	t.Setenv("DEEPSEEK_API_KEY_A", "sk")
	if !Configured("deepseek", "A") {
		t.Error("deepseek with key should be configured")
	}

	t.Setenv("OLLAMA_BASE_URL", "http://localhost:11434/v1")
	if !Configured("ollama", "A") {
		t.Error("local provider with base URL should be configured")
	}
}
