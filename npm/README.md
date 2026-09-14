# npm distribution

`intent-lang` is a **thin distribution wrapper** around the Go binary.
There is deliberately no JavaScript reimplementation — the Go binary is the
single source of truth, and this package just ships it.

## Layout

```
npm/
  intent-lang/          # main package (intent-lang)
    package.json        # bin: il, optionalDependencies: platform packages
    bin/il.js           # launcher: resolves + execs the platform binary
  platforms/            # generated (gitignored): one package per platform
    linux-x64/  linux-arm64/  darwin-x64/  darwin-arm64/  win32-x64/  win32-arm64/
  scripts/build.mjs     # cross-compiles the Go binary and writes platform packages
```

## Build locally

```sh
node npm/scripts/build.mjs            # uses the main package's version
VERSION=0.2.0 node npm/scripts/build.mjs   # or override (bash)
```

This runs `go build` with `GOOS/GOARCH` for each target (`CGO_ENABLED=0`) and
writes `npm/platforms/<target>/bin/il[.exe]` plus each platform `package.json`.

## Publish (CI)

`.github/workflows/npm.yml` builds and publishes on every `v*` tag:

1. platform packages first (`npm publish npm/platforms/<target> --access public`)
2. then the main package (`npm publish npm/intent-lang --access public`)

Requires a repository secret **`NPM_TOKEN`** (an npm automation token with
publish rights for the `@lovepk` scope).

To publish manually:

```sh
VERSION=0.1.0 node npm/scripts/build.mjs
npm login
for d in npm/platforms/*/; do npm publish "$d" --access public; done
npm publish npm/intent-lang --access public
```
