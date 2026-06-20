#!/usr/bin/env bash
set -euo pipefail

HOST="${GO_BLOG_SSH_HOST:-root@114.115.214.86}"
REMOTE_DIR="${GO_BLOG_REMOTE_CMD_DIR:-/data/program/go/go_blog/cmd}"
KNOWN_HOSTS="${GO_BLOG_KNOWN_HOSTS:-/tmp/codex_known_hosts_go_blog_trace}"
PASSWORD="${GO_BLOG_SSH_PASSWORD:-}"
SESSION=""
REQUEST=""
USER_ID=""
LINES="${LINES:-800}"

usage() {
  cat <<'USAGE'
Usage:
  GO_BLOG_SSH_PASSWORD='...' scripts/collect_codegen_trace.sh --session acp_xxx [--request cmd_start_xxx] [--user ztt]

Collects a read-only codegen trace across gateway, cmd-agent, app-agent and related logs.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --session)
      SESSION="${2:-}"
      shift 2
      ;;
    --request)
      REQUEST="${2:-}"
      shift 2
      ;;
    --user)
      USER_ID="${2:-}"
      shift 2
      ;;
    --lines)
      LINES="${2:-800}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "$SESSION" && -z "$REQUEST" ]]; then
  echo "either --session or --request is required" >&2
  usage >&2
  exit 2
fi
if [[ -z "$PASSWORD" ]]; then
  echo "GO_BLOG_SSH_PASSWORD is required" >&2
  exit 1
fi
if ! command -v sshpass >/dev/null 2>&1; then
  echo "sshpass is required" >&2
  exit 1
fi

REMOTE_SCRIPT='
set -euo pipefail
cd "$1"
session="$2"
request="$3"
user_id="$4"
lines="$5"

pattern="codegen_stream|acp_stream|stream_event|task_complete|tool_call|tool_result|inbound app message|route app notify|notify payload|enqueue app push|pushed message|ack message|missing route|forward"
if [[ -n "$session" ]]; then
  pattern="$session|$pattern"
fi
if [[ -n "$request" ]]; then
  pattern="$request|$pattern"
fi
if [[ -n "$user_id" ]]; then
  pattern="$user_id|$pattern"
fi

echo "== trace input =="
echo "session=$session request=$request user=$user_id"

echo
echo "== process snapshot =="
ps -ef | grep -E "(app-agent|cmd-agent|acp-agent|gateway)" | grep -v grep || true

echo
echo "== gateway route =="
grep -n -E "$pattern" gateway/gateway.log 2>/dev/null | tail -n "$lines" || true

echo
echo "== cmd-agent route =="
grep -n -E "$pattern" cmd-agent/cmd-agent.log 2>/dev/null | tail -n "$lines" || true

echo
echo "== app-agent websocket =="
grep -n -E "$pattern" app-agent/app-agent.log 2>/dev/null | tail -n "$lines" || true

echo
echo "== related agent logs =="
grep -R -n -E "$pattern" -- acp-agent/*.log llm-agent/*.log cron-agent/*.log hermes-agent/*.log 2>/dev/null | tail -n "$lines" || true

echo
echo "== diagnosis checklist =="
if grep -q -E "acp_stream|stream_event" gateway/gateway.log app-agent/app-agent.log cmd-agent/cmd-agent.log 2>/dev/null; then
  echo "gateway_or_agent_stream_seen=yes"
else
  echo "gateway_or_agent_stream_seen=no"
fi
if [[ -n "$session" ]] && grep -q "codegen_stream:$session" app-agent/app-agent.log 2>/dev/null; then
  echo "app_agent_codegen_push_seen=yes"
else
  echo "app_agent_codegen_push_seen=no"
fi
if [[ -n "$session" ]] && grep -q "ack message id=codegen_stream:$session" app-agent/app-agent.log 2>/dev/null; then
  echo "flutter_ack_seen=yes"
else
  echo "flutter_ack_seen=no"
fi
'

sshpass -p "$PASSWORD" ssh \
  -o StrictHostKeyChecking=no \
  -o UserKnownHostsFile="$KNOWN_HOSTS" \
  "$HOST" \
  "bash -s -- '$REMOTE_DIR' '$SESSION' '$REQUEST' '$USER_ID' '$LINES'" <<<"$REMOTE_SCRIPT"
