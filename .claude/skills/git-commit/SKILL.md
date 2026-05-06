---
name: git-commit
description: Generate a git commit with a concise Chinese commit message, stage all files (excluding binaries), commit with DeepSeek co-author, and push to origin. Use this skill whenever the user asks to commit code,提交代码,生成commit,提交并推送,commit and push, or any request involving creating a git commit.
---

# Git Commit Skill

Generate a well-crafted git commit following this project's conventions and push to origin.

## Commit Message Style

Follow the existing repo pattern:
- Use Chinese descriptions
- Prefix with `feat:` for features/enhancements or `fix:` for bug fixes
- Separate multiple topics with Chinese comma (`、`)
- Keep it concise — one line summarizing all changes
- Focus on the "why" and what changed at a high level

**Examples from this repo:**
- `feat: cortana主动播报决策、app-agent会话同步、flutter播报队列及委托令牌刷新`
- `fix: cortana proactive event 静默跳过路径添加日志`
- `feat: acp-agent进程树管理、cortana-agent播报代理、app-agent工具调用桥接及flutter播报队列`

## Workflow

### 1. Gather Information
Run these in parallel:
```bash
git status
git diff --stat
git log --oneline -5
```

### 2. Analyze and Generate Message
- Read the diff stats to understand which agents/modules changed
- Look at file names and directories to identify the components involved
- Check untracked files for new features/packages
- Generate a single concise Chinese commit message that covers all major changes
- If changes span multiple agents, list them with `、` separators

### 3. Stage Files
- `git add` all modified and untracked source files
- **Exclude binary files** — compiled binaries (e.g., `cortana-agent` without extension in `cmd/cortana-agent/`), `.exe` files, `.dylib`, `.so`, etc.
- **Exclude credential/token files** — any file that may contain secrets, tokens, API keys, or credentials:
  - `.env` files, `credentials.*`, `secrets.*`, `tokens.*`
  - Runtime config files whose `.example` counterpart already exists in the repo (e.g., `image-agent.json` vs `image-agent.json.example`). Only commit the `.example` template, never the actual config.
  - Any JSON/YAML file containing fields like `token`, `api_key`, `secret`, `password`, `apiKey`
  - Before staging a config file, check its content — if unsure, skip it
- Use targeted `git add <path>` commands rather than `git add -A` or `git add .`

### 4. Commit
Use a heredoc to format the commit message:
```bash
git commit -m "$(cat <<'EOF'
feat: <Chinese description here>

Co-Authored-By: 🐋 DeepSeek v4 Pro <noreply@deepseek.com>
EOF
)"
```

### 5. Push
```bash
git push
```

## Important Rules

- Never commit binary files (compiled executables, `.o`, `.a`, `.dylib`, `.so`, `.exe`)
- Never commit credential/token files — check for `.env`, `credentials.*`, runtime config files (non-`.example` counterparts), or any file containing `token`/`api_key`/`secret`/`password` fields. When a `.example` template exists, only commit the template.
- Never use `git add -A` or `git add .` — always stage specific paths
- Never amend commits — always create new ones
- Never skip hooks (`--no-verify`, `--no-gpg-sign`)
- Always use `Co-Authored-By: 🐋 DeepSeek v4 Pro <noreply@deepseek.com>` as the co-author
- After committing, verify with `git status` that it succeeded
