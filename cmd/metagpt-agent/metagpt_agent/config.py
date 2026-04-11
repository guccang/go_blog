from __future__ import annotations

import json
from dataclasses import dataclass, field
from pathlib import Path


@dataclass
class LLMConfig:
    base_url: str = "https://api.openai.com/v1"
    api_key: str = ""
    model: str = "gpt-4o-mini"
    max_tokens: int = 8192
    temperature: float = 0.3
    timeout_sec: int = 180


@dataclass
class AgentConfig:
    gateway_url: str = "ws://127.0.0.1:9000/ws/uap"
    gateway_http: str = "http://127.0.0.1:9000"
    auth_token: str = ""
    agent_id: str = "metagpt-agent"
    agent_name: str = "MetaGPT Agent"
    agent_type: str = "llm_mcp"
    description: str = "MetaGPT-based UAP task agent"
    llm: LLMConfig = field(default_factory=LLMConfig)
    system_prompt_prefix: str = (
        "你是基于 MetaGPT 的工程型智能体。优先直接完成任务，不要编造执行结果。"
    )
    max_tool_iterations: int = 16
    tool_call_timeout_sec: int = 300
    max_concurrent: int = 2
    task_queue_size: int = 16
    discovery_interval_sec: int = 30
    heartbeat_interval_sec: int = 15
    session_dir: str = "agent_sessions_metagpt"
    chat_session_dir: str = "chat_sessions_metagpt"
    workspace_dir: str = "workspace"

    @classmethod
    def default(cls) -> "AgentConfig":
        return cls()

    @classmethod
    def load(cls, path: str) -> "AgentConfig":
        config_path = Path(path)
        if not config_path.exists():
            return cls.default()

        raw = json.loads(config_path.read_text(encoding="utf-8"))
        llm_raw = raw.pop("llm", {})
        config = cls(**raw)
        config.llm = LLMConfig(**llm_raw)
        return config

    def ensure_dirs(self) -> None:
        Path(self.session_dir).mkdir(parents=True, exist_ok=True)
        Path(self.chat_session_dir).mkdir(parents=True, exist_ok=True)
        Path(self.workspace_dir).mkdir(parents=True, exist_ok=True)
