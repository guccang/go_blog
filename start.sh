#!/bin/bash
cd "$(dirname "$0")"
BASEDIR="$(pwd)"

nohup ./go_blog "${BASEDIR}/blogs_txt/ztt/sys_conf.md" > /dev/null 2>&1 &
echo "go_blog started (pid: $!)"

for svc in gateway codegen-agent deploy-agent wechat-agent llm-mcp-agent; do
    (cd "${BASEDIR}/cmd/${svc}" && nohup "./${svc}" > ${svc}.log &)
    echo "${svc} started"
done

echo "所有服务已启动"
