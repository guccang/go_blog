# Repository Guidelines

## Project Structure & Module Organization
This repository is a Go monorepo centered on `cmd/`. The main app lives in `cmd/blog-agent/`, with business modules under `cmd/blog-agent/pkgs/` such as `blog/`, `http/`, `auth/`, `llm/`, and `persistence/`. Other agents, including `cmd/gateway/`, `cmd/llm-agent/`, `cmd/wechat-agent/`, and `cmd/deploy-agent/`, run as separate services. Runtime content is stored in `blogs_txt/`, templates in `templates/`, static assets in `statics/`, and architecture notes in `docs/`.

## Build, Test, and Development Commands
Use Go 1.24.x and Redis.

```bash
cd cmd/blog-agent && go mod tidy && go build
cd cmd/blog-agent && go test ./...
cd cmd/blog-agent && go test -v ./pkgs/encryption -run TestAesSimpleEncrypt
cd cmd/blog-agent && go vet ./...
cd cmd/blog-agent && ./scripts/start_redis.sh
cd cmd/blog-agent && ./go_blog ../blogs_txt/sys_conf.md
```

Build other agents from their own directories, for example `cd cmd/gateway && go build -o gateway`.

## Coding Style & Naming Conventions
Follow standard Go formatting with `gofmt`; keep imports grouped as standard library, third-party, then internal modules. Shared mutable state must be guarded with `sync.RWMutex`. New multi-tenant APIs should use the `WithAccount` suffix, for example `GetBlogWithAccount`. Exported names use PascalCase; private helpers use camelCase. Storage types usually end with `Store`, managers with `Manager`, and enum-like constants often use an `E` prefix. Prefer Chinese comments when adding non-obvious logic.

## Testing Guidelines
Use Go's built-in `testing` package. Keep tests next to the package they cover and name them `*_test.go` with functions like `TestXxx`. Run focused tests for a module before broad `go test ./...`. Add regression tests for parsing, storage, routing, and multi-account behavior when touching those paths.

## Commit & Pull Request Guidelines
Recent history follows Conventional Commit prefixes such as `feat:`, `fix:`, and `refactor:`; keep subjects short and specific, often with a brief Chinese description. PRs should include scope, affected agents or packages, config changes, test commands run, and screenshots for UI/template changes. Link the relevant issue or task when available.

## Security & Configuration Tips
Do not commit API keys, tokens, or private config files. Use the example JSON configs in agent directories, and keep secrets out of `blogs_txt/` and checked-in logs. Validate Redis-dependent flows locally before merging.
