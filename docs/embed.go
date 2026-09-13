package docs

import _ "embed"

// IntentSpec is the embedded IL language specification (v2.0).
//
//go:embed intent-spec.md
var IntentSpec string

// IntentAgent is the embedded agent integration contract.
//
//go:embed intent-agent.md
var IntentAgent string

// IntentKeywords is the embedded human-readable keyword reference.
//
//go:embed intent-keywords.md
var IntentKeywords string
