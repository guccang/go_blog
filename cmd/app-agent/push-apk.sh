#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
用法:
  ./push-apk.sh (-u <to_user> | -g <group_id>) -f <apk_path> [options]

必填参数:
  -u, --user        目标 Flutter 用户 ID
  -g, --group       目标群组 ID（会广播给群内所有用户，排除群机器人）
  -f, --file        APK 文件路径

可选参数:
  -s, --server      app-agent 地址，默认 http://127.0.0.1:9002
  -t, --token       app-agent receive_token，也可用环境变量 APP_AGENT_TOKEN
  -m, --message     推送文案，默认 "新的安装包已下发，点击安装"
  -h, --help        显示帮助

示例:
  ./push-apk.sh -u ztt -f ./build/app-release.apk
  ./push-apk.sh -g team-alpha -f ./build/app-release.apk
  ./push-apk.sh -u ztt -f ./build/app-release.apk -s http://127.0.0.1:9002 -t your_token
EOF
}

SERVER_URL="${APP_AGENT_SERVER:-http://127.0.0.1:9002}"
APP_AGENT_TOKEN="${APP_AGENT_TOKEN:-}"
TO_USER=""
GROUP_ID=""
APK_PATH=""
MESSAGE="新的安装包已下发，点击安装"

while [[ $# -gt 0 ]]; do
  case "$1" in
    -u|--user)
      TO_USER="${2:-}"
      shift 2
      ;;
    -g|--group)
      GROUP_ID="${2:-}"
      shift 2
      ;;
    -f|--file)
      APK_PATH="${2:-}"
      shift 2
      ;;
    -s|--server)
      SERVER_URL="${2:-}"
      shift 2
      ;;
    -t|--token)
      APP_AGENT_TOKEN="${2:-}"
      shift 2
      ;;
    -m|--message)
      MESSAGE="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "未知参数: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ -n "${TO_USER}" && -n "${GROUP_ID}" ]]; then
  echo "不能同时指定用户和群组: -u/--user 与 -g/--group" >&2
  usage
  exit 1
fi

if [[ -z "${TO_USER}" && -z "${GROUP_ID}" ]]; then
  echo "缺少目标用户或群组: -u/--user 或 -g/--group" >&2
  usage
  exit 1
fi

if [[ -z "${APK_PATH}" ]]; then
  echo "缺少 APK 路径: -f/--file" >&2
  usage
  exit 1
fi

if [[ ! -f "${APK_PATH}" ]]; then
  echo "APK 文件不存在: ${APK_PATH}" >&2
  exit 1
fi

if [[ "${APK_PATH##*.}" != "apk" && "${APK_PATH##*.}" != "APK" ]]; then
  echo "文件不是 .apk: ${APK_PATH}" >&2
  exit 1
fi

UPLOAD_URL="${SERVER_URL%/}/api/app/upload-apk"

CURL_ARGS=(
  --fail-with-body
  --show-error
  -X POST
  "${UPLOAD_URL}"
  -F "content=${MESSAGE}"
  -F "file=@${APK_PATH};filename=$(basename "${APK_PATH}");type=application/vnd.android.package-archive"
)

if [[ -n "${TO_USER}" ]]; then
  CURL_ARGS+=(-F "to_user=${TO_USER}")
fi
if [[ -n "${GROUP_ID}" ]]; then
  CURL_ARGS+=(-F "group_id=${GROUP_ID}")
fi

if [[ -n "${APP_AGENT_TOKEN}" ]]; then
  CURL_ARGS+=(-H "X-App-Agent-Token: ${APP_AGENT_TOKEN}")
fi

echo "上传 APK 到 ${UPLOAD_URL}"
if [[ -n "${TO_USER}" ]]; then
  echo "目标用户: ${TO_USER}"
else
  echo "目标群组: ${GROUP_ID}"
fi
echo "文件路径: ${APK_PATH}"
echo "上传进度:"
echo

curl "${CURL_ARGS[@]}"
echo
