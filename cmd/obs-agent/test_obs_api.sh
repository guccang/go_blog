#!/usr/bin/env bash
# obs-agent API 接口测试脚本
# 用法: ./test_obs_api.sh [obs-agent地址] [token]
# 示例: ./test_obs_api.sh http://localhost:9004 my-secret-token
#       ./test_obs_api.sh http://192.168.1.100:9004 my-secret-token

set -euo pipefail

BASE_URL="${1:-http://localhost:9004}"
TOKEN="${2:-}"
PASS=0
FAIL=0
UPLOADED_KEY=""

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log()  { echo -e "${CYAN}[$(date '+%H:%M:%S')]${NC} $*"; }
pass() { PASS=$((PASS + 1)); echo -e "  ${GREEN}PASS${NC} $*"; }
fail() { FAIL=$((FAIL + 1)); echo -e "  ${RED}FAIL${NC} $*"; }
warn() { echo -e "  ${YELLOW}WARN${NC} $*"; }

auth_header() {
  if [ -n "$TOKEN" ]; then
    echo "Authorization: Bearer $TOKEN"
  else
    echo "X-No-Auth: true"
  fi
}

check_json_field() {
  local json="$1" field="$2" expected="$3"
  local actual
  actual=$(echo "$json" | jq -r ".$field // empty" 2>/dev/null)
  if [ "$actual" = "$expected" ]; then
    pass "$field=$actual"
    return 0
  else
    fail "$field expected='$expected' actual='$actual'"
    return 1
  fi
}

check_json_nonempty() {
  local json="$1" field="$2"
  local actual
  actual=$(echo "$json" | jq -r ".$field // empty" 2>/dev/null)
  if [ -n "$actual" ] && [ "$actual" != "null" ]; then
    pass "$field=$actual"
    return 0
  else
    fail "$field is empty"
    return 1
  fi
}

# ─── 前置检查 ───
log "obs-agent 地址: $BASE_URL"
if [ -z "$TOKEN" ]; then
  warn "未提供 token，如果 obs-agent 配置了 receive_token 则认证会失败"
fi

if ! command -v jq &>/dev/null; then
  echo -e "${RED}错误: 需要安装 jq (brew install jq / apt install jq)${NC}"
  exit 1
fi

# ─── 1. Health Check ───
log "1. Health Check"
RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" 2>&1)
HTTP_CODE=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | sed '$d')
if [ "$HTTP_CODE" = "200" ]; then
  pass "GET /health => $HTTP_CODE"
  check_json_field "$BODY" "status" "ok"
  OBS_ENABLED=$(echo "$BODY" | jq -r '.obs_enabled')
  log "  obs_enabled=$OBS_ENABLED"
  if [ "$OBS_ENABLED" != "true" ]; then
    warn "OBS 未启用，后续上传/下载测试可能失败"
  fi
else
  fail "GET /health => $HTTP_CODE"
  echo "  响应: $BODY"
fi

# ─── 2. 签名URL上传 (获取 signed PUT URL) ───
log "2. 签名URL上传 - POST /api/obs/upload"
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/obs/upload" \
  -H "$(auth_header)" \
  -H "Content-Type: application/json" \
  -d '{"file_name":"test_data.csv","content_type":"text/csv"}' 2>&1)
HTTP_CODE=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | sed '$d')
if [ "$HTTP_CODE" = "200" ]; then
  pass "POST /api/obs/upload => $HTTP_CODE"
  check_json_field "$BODY" "success" "true"
  check_json_field "$BODY" "method" "PUT"
  check_json_nonempty "$BODY" "upload_url"
  check_json_nonempty "$BODY" "object_key"
  check_json_nonempty "$BODY" "expires_at"
  SIGNED_PUT_URL=$(echo "$BODY" | jq -r '.upload_url')
  UPLOADED_KEY=$(echo "$BODY" | jq -r '.object_key')
  log "  object_key=$UPLOADED_KEY"

  # 用签名URL实际上传数据
  log "  使用签名URL上传测试数据..."
  echo "id,name,value" > /tmp/obs_test_data.csv
  echo "1,test,hello" >> /tmp/obs_test_data.csv
  PUT_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$SIGNED_PUT_URL" \
    -H "Content-Type: text/csv" \
    --data-binary @/tmp/obs_test_data.csv 2>&1)
  if [ "$PUT_CODE" = "200" ]; then
    pass "PUT signed_url => $PUT_CODE (数据已上传到OBS)"
  else
    fail "PUT signed_url => $PUT_CODE"
    warn "签名URL上传失败，可能是URL已过期或OBS配置问题"
  fi
  rm -f /tmp/obs_test_data.csv
else
  fail "POST /api/obs/upload => $HTTP_CODE"
  echo "  响应: $BODY"
fi

# ─── 3. 签名URL上传 - 自定义 object_key ───
log "3. 签名URL上传 - 自定义 object_key"
CUSTOM_KEY="test/custom_$(date +%s).txt"
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/obs/upload" \
  -H "$(auth_header)" \
  -H "Content-Type: application/json" \
  -d "{\"file_name\":\"custom.txt\",\"object_key\":\"$CUSTOM_KEY\",\"content_type\":\"text/plain\"}" 2>&1)
HTTP_CODE=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | sed '$d')
if [ "$HTTP_CODE" = "200" ]; then
  pass "POST /api/obs/upload (custom key) => $HTTP_CODE"
  ACTUAL_KEY=$(echo "$BODY" | jq -r '.object_key')
  log "  请求key=$CUSTOM_KEY 实际key=$ACTUAL_KEY"
else
  fail "POST /api/obs/upload (custom key) => $HTTP_CODE"
  echo "  响应: $BODY"
fi

# ─── 4. 代理上传 (multipart) ───
log "4. 代理上传 - POST /api/obs/proxy-upload"
echo "proxy upload test content $(date)" > /tmp/obs_proxy_test.txt
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/obs/proxy-upload" \
  -H "$(auth_header)" \
  -F "file=@/tmp/obs_proxy_test.txt" \
  -F "content_type=text/plain" 2>&1)
HTTP_CODE=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | sed '$d')
if [ "$HTTP_CODE" = "200" ]; then
  pass "POST /api/obs/proxy-upload => $HTTP_CODE"
  check_json_field "$BODY" "success" "true"
  check_json_nonempty "$BODY" "object_key"
  check_json_nonempty "$BODY" "size"
  PROXY_KEY=$(echo "$BODY" | jq -r '.object_key')
  PROXY_SIZE=$(echo "$BODY" | jq -r '.size')
  log "  object_key=$PROXY_KEY size=$PROXY_SIZE"
  # 后续用这个 key 测试 info 和 delete
  if [ -z "$UPLOADED_KEY" ]; then
    UPLOADED_KEY="$PROXY_KEY"
  fi
else
  fail "POST /api/obs/proxy-upload => $HTTP_CODE"
  echo "  响应: $BODY"
fi
rm -f /tmp/obs_proxy_test.txt

# ─── 5. 列表查询 ───
log "5. 列表查询 - GET /api/obs/list"
RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/api/obs/list?prefix=upload/" \
  -H "$(auth_header)" 2>&1)
HTTP_CODE=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | sed '$d')
if [ "$HTTP_CODE" = "200" ]; then
  pass "GET /api/obs/list => $HTTP_CODE"
  check_json_field "$BODY" "success" "true"
  OBJ_COUNT=$(echo "$BODY" | jq '.objects | length' 2>/dev/null)
  IS_TRUNCATED=$(echo "$BODY" | jq -r '.is_truncated' 2>/dev/null)
  log "  对象数量=$OBJ_COUNT truncated=$IS_TRUNCATED"
  if [ "$OBJ_COUNT" -gt 0 ] 2>/dev/null; then
    FIRST_KEY=$(echo "$BODY" | jq -r '.objects[0].key')
    FIRST_SIZE=$(echo "$BODY" | jq -r '.objects[0].size')
    log "  首个对象: key=$FIRST_KEY size=$FIRST_SIZE"
    pass "列表返回 $OBJ_COUNT 个对象"
  else
    warn "列表为空，prefix=upload/ 下没有对象"
  fi
else
  fail "GET /api/obs/list => $HTTP_CODE"
  echo "  响应: $BODY"
fi

# ─── 6. 对象信息查询 ───
log "6. 对象信息查询 - GET /api/obs/info"
if [ -n "$UPLOADED_KEY" ]; then
  RESP=$(curl -s -w "\n%{http_code}" \
    "$BASE_URL/api/obs/info?object_key=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$UPLOADED_KEY'))" 2>/dev/null || echo "$UPLOADED_KEY")" \
    -H "$(auth_header)" 2>&1)
  HTTP_CODE=$(echo "$RESP" | tail -1)
  BODY=$(echo "$RESP" | sed '$d')
  if [ "$HTTP_CODE" = "200" ]; then
    pass "GET /api/obs/info => $HTTP_CODE"
    check_json_field "$BODY" "success" "true"
    check_json_nonempty "$BODY" "object_key"
    check_json_nonempty "$BODY" "size"
    check_json_nonempty "$BODY" "content_type"
    check_json_nonempty "$BODY" "last_modified"
    INFO_SIZE=$(echo "$BODY" | jq -r '.size')
    INFO_TYPE=$(echo "$BODY" | jq -r '.content_type')
    log "  key=$UPLOADED_KEY size=$INFO_SIZE type=$INFO_TYPE"
  else
    fail "GET /api/obs/info => $HTTP_CODE"
    echo "  响应: $BODY"
  fi
else
  warn "跳过: 没有可查询的 object_key（前面的上传可能失败了）"
fi

# ─── 7. 删除对象 ───
log "7. 删除对象 - POST /api/obs/delete"
if [ -n "$UPLOADED_KEY" ]; then
  RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/obs/delete" \
    -H "$(auth_header)" \
    -H "Content-Type: application/json" \
    -d "{\"object_key\":\"$UPLOADED_KEY\"}" 2>&1)
  HTTP_CODE=$(echo "$RESP" | tail -1)
  BODY=$(echo "$RESP" | sed '$d')
  if [ "$HTTP_CODE" = "200" ]; then
    pass "POST /api/obs/delete => $HTTP_CODE"
    check_json_field "$BODY" "success" "true"
    log "  已删除: $UPLOADED_KEY"
  else
    fail "POST /api/obs/delete => $HTTP_CODE"
    echo "  响应: $BODY"
  fi

  # 验证删除后查询应该失败
  log "  验证删除后 info 查询..."
  RESP=$(curl -s -w "\n%{http_code}" \
    "$BASE_URL/api/obs/info?object_key=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$UPLOADED_KEY'))" 2>/dev/null || echo "$UPLOADED_KEY")" \
    -H "$(auth_header)" 2>&1)
  HTTP_CODE=$(echo "$RESP" | tail -1)
  if [ "$HTTP_CODE" != "200" ]; then
    pass "删除后 info 返回 $HTTP_CODE (符合预期)"
  else
    warn "删除后 info 仍返回 200，OBS 可能有延迟"
  fi
else
  warn "跳过: 没有可删除的 object_key"
fi

# ─── 8. 错误场景测试 ───
log "8. 错误场景测试"

# 8a. 缺少 file_name
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/obs/upload" \
  -H "$(auth_header)" \
  -H "Content-Type: application/json" \
  -d '{"content_type":"text/csv"}' 2>&1)
HTTP_CODE=$(echo "$RESP" | tail -1)
if [ "$HTTP_CODE" = "400" ]; then
  pass "缺少 file_name => 400"
else
  fail "缺少 file_name => $HTTP_CODE (期望 400)"
fi

# 8b. 缺少 object_key (delete)
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/obs/delete" \
  -H "$(auth_header)" \
  -H "Content-Type: application/json" \
  -d '{}' 2>&1)
HTTP_CODE=$(echo "$RESP" | tail -1)
if [ "$HTTP_CODE" = "400" ]; then
  pass "delete 缺少 object_key => 400"
else
  fail "delete 缺少 object_key => $HTTP_CODE (期望 400)"
fi

# 8c. 错误的 HTTP 方法
RESP=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/api/obs/upload" \
  -H "$(auth_header)" 2>&1)
HTTP_CODE=$(echo "$RESP" | tail -1)
if [ "$HTTP_CODE" = "405" ]; then
  pass "GET /api/obs/upload => 405"
else
  fail "GET /api/obs/upload => $HTTP_CODE (期望 405)"
fi

# 8d. 未授权 (如果配置了 token)
if [ -n "$TOKEN" ]; then
  RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/obs/upload" \
    -H "Authorization: Bearer wrong-token" \
    -H "Content-Type: application/json" \
    -d '{"file_name":"test.txt"}' 2>&1)
  HTTP_CODE=$(echo "$RESP" | tail -1)
  if [ "$HTTP_CODE" = "401" ]; then
    pass "错误 token => 401"
  else
    fail "错误 token => $HTTP_CODE (期望 401)"
  fi
fi

# ─── 汇总 ───
echo ""
echo -e "${CYAN}════════════════════════════════════════${NC}"
echo -e "  ${GREEN}PASS: $PASS${NC}  ${RED}FAIL: $FAIL${NC}"
echo -e "${CYAN}════════════════════════════════════════${NC}"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
