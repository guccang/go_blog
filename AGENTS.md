# Repository Guidelines

强约束：所有文件读写一律使用 UTF‑8 （无 BOM ）。禁止使用默认编码、GBK 、ANSI 。
执行任何写文件命令前，必须检查并确认控制台编码为 UTF‑8 （ chcp 65001 ），并在读写时显式指定 UTF‑8 。
如发现中文乱码（例如“绔炶禌 Service 涓氬姟灞傚鐞?”），不得直接修乱码文本，必须先确定原文（从正确源文件/历史版本/上下文）再替换。
若无法确认原文，必须先询问再修改。
违反以上规则将导致编码再次损坏，务必严格遵守。

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

## Security & Configuration Tips
Do not commit API keys, tokens, or private config files. Use the example JSON configs in agent directories, and keep secrets out of `blogs_txt/` and checked-in logs. Validate Redis-dependent flows locally before merging.
