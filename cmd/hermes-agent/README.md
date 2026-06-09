# hermes-agent

`hermes-agent` 将 Nous Research Hermes 的原生 `AIAgent`、工具、技能、记忆、
上下文压缩、子 Agent 和 cron 能力接入 go_blog UAP 网络。

默认使用仓库内锁定的 `vendor/hermes_runtime`，运行机器不需要安装 Hermes。
本目录只维护 UAP 适配、运行时选择、配置隔离和 cron 回推，Hermes 核心源码保持
可同步的独立模块。

## 运行方式

首次启动会在 `.runtime/venv` 自动创建 Python 3.11 虚拟环境，并根据
`requirements.lock` 下载依赖。机器需要满足以下任一条件：

- 已安装 `uv`，启动器会自动准备 Python 3.11。
- 已安装 Python 3.11+，并可通过 `PYTHON_BOOTSTRAP` 指定命令。

```bash
cd cmd/hermes-agent
cp hermes-agent.json.example hermes-agent.json
./hermes-agent --config hermes-agent.json --check-runtime
./hermes-agent --config hermes-agent.json
```

`--check-runtime` 会真实导入内置 `AIAgent` 并检查原生工具集。正常输出应包含
`'mode': 'embedded'`、`'ready': True` 和 `'cronjob': True`。

让 Flutter 消息进入 Hermes 时，把 `app-agent` 配置中的 `llm_agent_id`
改为 `hermes-agent`。Flutter 继续使用原有 `POST /api/app/message` 和
`GET /ws/app` 接口。

## 模块边界

- `vendor/hermes_runtime/`：锁定的 Hermes 原生运行时源码和许可证。
- `native_runtime.py`：选择、初始化并检查 embedded/external 运行时。
- `runtime.py`：UAP 消息适配、任务队列、会话续接和执行结果回推。
- `cron_bridge.py`：运行 Hermes 原生 cron scheduler，并将 App 来源任务结果
  回推给原始 App 用户。
- `config.py`：配置解析、路径解析和独立 `HERMES_HOME` 初始化。

示例配置将 Hermes 运行数据写入 `cmd/hermes-agent/state/hermes/`，对话记录写入
配置的 `session_dir`，不会依赖或修改用户目录下的 `~/.hermes`。

## 配置

- `runtime_mode`：默认 `embedded`；显式设为 `external` 才使用外部 Hermes。
- `embedded_source`：仓库内 Hermes 运行时路径。
- `hermes_source`：`external` 模式的 Hermes 源码路径。
- `hermes_home`：独立的 Hermes 配置、记忆、技能和 cron 数据目录。
- `workspace_dir`：Hermes 执行任务的工作目录。
- `session_dir`：按 Flutter 用户或 UAP task 保存的对话历史。
- `model/provider/base_url/api_key`：模型连接配置。
- `native_config`：透传并合并到 Hermes 原生配置的扩展字段。
- `enabled_toolsets/disabled_toolsets`：限制本轮可使用的工具集。

## 外部运行时

外部 Hermes 仅用于开发对比或同步前验证：

```json
{
  "runtime_mode": "external",
  "hermes_source": "/path/to/hermes-agent"
}
```

## 更新内置 Hermes

从一份已下载的 Hermes 源码同步允许的原生模块，并重新生成校验清单：

```bash
./sync-hermes-runtime.sh /path/to/hermes-agent
```

同步信息记录在 `vendor/hermes_runtime/VENDOR_INFO.json`，文件校验值记录在
`vendor/hermes_runtime/SHA256SUMS`。同步后应重新生成 `requirements.lock`
并运行全部验证。

## 验证

```bash
./hermes-agent --genconf --config /tmp/hermes-agent.json
./.runtime/venv/bin/python -m unittest discover -s tests -v
./hermes-agent --config hermes-agent.json.example --check-runtime
```

## 常见问题

### DeepSeek API Key 缺失错误

如果遇到 `Provider deepseek is set in config.yaml but no API key was found` 错误：

**原因**：Hermes Runtime 在执行 Cron job 时从 `state/hermes/.env` 加载环境变量，但该文件中没有 `DEEPSEEK_API_KEY`。

**解决方案**：

#### 方案一：自动修复（推荐）

运行修复脚本，它会从 `hermes-agent.json` 读取 API key 并写入 `state/hermes/.env`：

```bash
cd cmd/hermes-agent
./fix-deepseek-env.sh
```

脚本会：
- 检查 `hermes-agent.json` 配置
- 创建 `state/hermes/.env` 文件
- 写入 `DEEPSEEK_API_KEY` 环境变量
- 验证配置正确性

#### 方案二：手动修复

1. **确保 `hermes-agent.json` 中配置了 API key**：
   ```json
   {
     "provider": "deepseek",
     "model": "deepseek-v4-flash",
     "api_key": "your-deepseek-api-key-here"
   }
   ```

2. **手动创建 `state/hermes/.env` 文件**：
   ```bash
   mkdir -p state/hermes
   echo 'DEEPSEEK_API_KEY="your-api-key-here"' > state/hermes/.env
   chmod 600 state/hermes/.env
   ```

3. **重启 hermes-agent 服务**

#### 方案三：切换到其他 provider

如果不想使用 deepseek，可以切换到 openrouter：

```json
{
  "provider": "openrouter",
  "model": "stepfun",
  "api_key": "your-openrouter-api-key"
}
```

然后运行 `./fix-deepseek-env.sh` 或重启服务（它会自动同步环境变量）。

#### 验证修复

1. 检查环境变量文件：
   ```bash
   cat state/hermes/.env
   ```

2. 重启服务并查看日志，确认 Cron job 不再报错
