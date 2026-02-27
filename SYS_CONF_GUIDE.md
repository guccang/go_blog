# Go Blog 系统配置说明

## 📋 概述

`sys_conf.md` 采用 `key=value` 格式，位于 `blogs_txt/用户名/sys_conf.md`。
系统启动时读取，大部分配置修改后需**重启服务**生效。

---

## ⚡ 快速配置（分级指南）

### 🟢 第一步：必填项（系统启动）

```ini
# 管理员账户
admin=你的用户名
pwd=你的密码

# 服务器
port=8888

# Redis
redis_ip=127.0.0.1
redis_port=6379
```

> 仅配置以上 5 项，系统即可启动。其余均为**可选功能**。

---

### 🟡 第二步：AI 功能（推荐）

配置任意一个 AI 模型即可使用 `/assistant` 和 `/agent`。

```ini
# DeepSeek（推荐，国内速度快）
deepseek_api_key=sk-xxxxxxxxxxxxxxxx
deepseek_api_url=https://api.deepseek.com/chat/completions

# OpenAI（可选，支持运行时切换）
openai_api_key=sk-xxxxxxxxxxxxxxxx
openai_api_url=https://api.openai.com/v1/chat/completions

# 通义千问（可选）
qwen_api_key=sk-xxxxxxxxxxxxxxxx
qwen_api_url=https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions

# 模型降级链（当主模型失败时依次尝试）
llm_fallback_models=["openai","qwen"]
```

---

### 🔵 第三步：通知推送（按需开启）

#### 📧 邮件通知

```ini
email_from=你的邮箱@gmail.com
email_password=应用专用密码
smtp_host=smtp.gmail.com
smtp_port=587
email_to=接收通知@qq.com
```

#### 💬 企业微信机器人

```ini
# 推送通知到企业微信群（仅需 webhook 即可）
wechat_webhook=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=你的key

# 接收企业微信指令（可选，需自建应用）
wechat_corp_id=wwxxxxxxxxxxxxxxxx
wechat_agent_id=1000002
wechat_secret=你的应用secret
wechat_token=你的回调token
wechat_encoding_aes_key=你的加密key

# 企业微信用户 ID ≠ go_blog 账号，始终使用管理员账号调用 MCP 工具
```

#### 📱 短信通知

```ini
sms_phone=13800138000
sms_send_url=https://push.spug.cc/send/你的key
```

---

### ⚪ 第四步：进阶配置（可选）

#### 内容管理

```ini
# 公开标签（|分隔）
publictags=技术|生活|读书

# 日记关键词（含关键词的博客需密码访问）
diary_keywords=日记_|私人日记_|个人记录_
diary_password=你的日记密码

# 自动添加日期后缀的标题前缀
title_auto_add_date_suffix=每日任务|锻炼|日记

# 系统文件（不显示在博客列表）
sysfiles=sys_conf|help
```

#### 显示设置

```ini
main_show_blogs=67          # 主页显示博客数量
help_blog_name=help         # 帮助页面博客名
max_blog_comments=100       # 最大评论数
```

#### 路径配置

```ini
templates_path=./templates
statics_path=./statics
download_path=/data/release/blogs
recycle_path=./recycle
```

#### 分享设置

```ini
share_days=7                # 分享链接有效天数
```

#### AI 高级设置

```ini
assistant_save_mcp_result=true    # 是否保存 MCP 工具调用结果
```

---

## 📊 配置项速查表

| 分类 | 配置项 | 必填 | 说明 |
|------|--------|:----:|------|
| **服务器** | `port` | ✅ | HTTP 端口 |
| **认证** | `admin` / `pwd` | ✅ | 管理员账号密码 |
| **Redis** | `redis_ip` / `redis_port` / `redis_pwd` | ✅ | 缓存数据库 |
| **AI-DeepSeek** | `deepseek_api_key` / `deepseek_api_url` | ⭐ | 推荐 |
| **AI-OpenAI** | `openai_api_key` / `openai_api_url` | — | 可选备用 |
| **AI-Qwen** | `qwen_api_key` / `qwen_api_url` | — | 可选备用 |
| **AI-降级** | `llm_fallback_models` | — | 模型降级链 |
| **邮件** | `email_from` / `email_password` / `smtp_host` / `smtp_port` / `email_to` | — | 邮件通知 |
| **企业微信** | `wechat_webhook` | — | 群机器人推送 |
| **企业微信回调** | `wechat_corp_id` / `wechat_token` / `wechat_encoding_aes_key` 等 | — | 接收指令 |
| **短信** | `sms_phone` / `sms_send_url` | — | 短信通知 |
| **内容** | `publictags` / `diary_keywords` / `diary_password` 等 | — | 博客管理 |
| **显示** | `main_show_blogs` / `help_blog_name` | — | 页面显示 |
| **路径** | `templates_path` / `statics_path` / `download_path` | — | 文件路径 |
| **分享** | `share_days` | — | 分享链接有效期 |
| **AI高级** | `assistant_save_mcp_result` | — | MCP 结果保存 |

---

## 📝 最小配置示例

以下是一个可以直接使用的最小配置文件：

```ini
# === 必填 ===
admin=myname
pwd=MyStr0ngP@ss
port=8888
redis_ip=127.0.0.1
redis_port=6379

# === AI（推荐）===
deepseek_api_key=sk-xxxxxx
deepseek_api_url=https://api.deepseek.com/chat/completions
```

---

## ⚠️ 安全提醒

1. **密码**: 生产环境务必更改默认密码
2. **API Key**: 妥善保管，不要提交到 Git
3. **文件权限**: `chmod 600 blogs_txt/*/sys_conf.md`
4. **HTTPS**: 生产环境建议使用 HTTPS（通过 Nginx 反向代理或直接配置证书）

---

## 🔍 故障排除

| 问题 | 排查方式 |
|------|----------|
| 端口被占用 | `netstat -tlnp \| grep 8888` |
| Redis 连不上 | `redis-cli -h 127.0.0.1 ping` |
| AI 不可用 | 检查 `deepseek_api_key` 是否正确 |
| 邮件发不出 | 检查 `smtp_host` 和应用专用密码 |
| 企业微信无通知 | 检查 `wechat_webhook` URL 是否有效 |

---

*最后更新: 2025年2月*
*文档版本: v2.0*