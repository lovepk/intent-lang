# Contributing

Thanks for your interest in intent-lang (IL) — an intent archive format and
protocol for agents.

## License

By contributing you agree that your contributions are licensed under the
[Apache License 2.0](LICENSE).

## Build & test

Requires Go (see `go.mod`).

```sh
go build ./...
go vet ./...
go test ./...        # unit tests are offline (no network / no API key)
```

The CLI:

```sh
go build -o il ./cmd/il
./il --help 2>/dev/null; ./il lint --archive testdata/calculator_llm_verified.il
```

Real-model commands (`chat`/`repro`/`accept`/`demo`, or MCP orchestrator tools)
need a provider configured via environment variables; see
`docs/intent-commands.md` →「Provider 配置」. Unit tests never require one.

## Changing the language spec

The spec (`docs/intent-spec.md`) is the product. Its version follows
semantic versioning (see spec §9):

- `minor` — only additive syntax elements; existing archives stay valid.
- `major` — breaking changes, with a migration note.

For a non-trivial change, open an issue first describing the motivation, then
send a PR that updates the spec **and** the parser/lint/tests together. Keep the
"record, don't interfere" principle (spec 原则 0) intact.

## Pull requests

- Keep changes focused; match the surrounding code style.
- Add or update tests (`go test ./...` must pass; keep tests offline).
- Update the relevant doc (`README.md` / `docs/*.md`) and `CHANGELOG.md`.
- Run `il fmt` on any `.il` files you touch.
