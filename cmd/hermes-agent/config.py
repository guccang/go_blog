"""Hermes UAP agent configuration."""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


@dataclass
class Config:
    gateway_url: str = "ws://127.0.0.1:9000/ws/uap"
    auth_token: str = ""
    agent_id: str = "hermes-agent"
    agent_name: str = "Hermes Agent"
    hermes_source: str = "~/.hermes/hermes-agent"
    workspace_dir: str = "."
    session_dir: str = "sessions"
    model: str = ""
    provider: str = ""
    base_url: str = ""
    api_key: str = ""
    system_prompt: str = (
        "你是通过 Flutter App 使用的 Hermes Agent。收到明确任务后直接执行，"
        "持续调用必要工具直到任务真实完成，并简洁汇报结果。"
    )
    max_iterations: int = 90
    max_concurrent: int = 3
    task_queue_size: int = 20
    enabled_toolsets: list[str] = field(default_factory=list)
    disabled_toolsets: list[str] = field(default_factory=list)

    @classmethod
    def load(cls, path: str | Path) -> "Config":
        config_path = Path(path)
        if not config_path.exists():
            return cls()
        with config_path.open("r", encoding="utf-8") as handle:
            raw = json.load(handle)
        known = {name for name in cls.__dataclass_fields__}
        return cls(**{key: value for key, value in raw.items() if key in known})

    def validate(self) -> None:
        if self.max_concurrent < 1:
            raise ValueError("max_concurrent must be at least 1")
        if self.task_queue_size < 1:
            raise ValueError("task_queue_size must be at least 1")
        if self.max_iterations < 1:
            raise ValueError("max_iterations must be at least 1")

    def write(self, path: str | Path) -> None:
        target = Path(path)
        target.parent.mkdir(parents=True, exist_ok=True)
        with target.open("w", encoding="utf-8", newline="\n") as handle:
            json.dump(self.__dict__, handle, ensure_ascii=False, indent=2)
            handle.write("\n")

    def agent_kwargs(self) -> dict[str, Any]:
        kwargs: dict[str, Any] = {
            "model": self.model,
            "max_iterations": self.max_iterations,
            "quiet_mode": True,
            "platform": "app",
            "load_soul_identity": True,
        }
        optional = {
            "provider": self.provider,
            "base_url": self.base_url,
            "api_key": self.api_key,
            "enabled_toolsets": self.enabled_toolsets or None,
            "disabled_toolsets": self.disabled_toolsets or None,
        }
        kwargs.update({key: value for key, value in optional.items() if value})
        return kwargs
