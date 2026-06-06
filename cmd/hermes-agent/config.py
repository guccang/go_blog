"""Hermes UAP agent configuration."""

from __future__ import annotations

import json
import os
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


@dataclass
class Config:
    gateway_url: str = "ws://127.0.0.1:9000/ws/uap"
    auth_token: str = ""
    agent_id: str = "hermes-agent"
    agent_name: str = "Hermes Agent"
    runtime_mode: str = "embedded"
    embedded_source: str = "vendor/hermes_runtime"
    hermes_source: str = "~/.hermes/hermes-agent"
    hermes_home: str = "state/hermes"
    workspace_dir: str = "."
    session_dir: str = "sessions"
    app_agent_id: str = "app-app-agent"
    cron_tick_seconds: int = 60
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
    native_config: dict[str, Any] = field(default_factory=dict)
    _config_dir: str = field(default=".", init=False, repr=False)

    @classmethod
    def load(cls, path: str | Path) -> "Config":
        config_path = Path(path)
        if not config_path.exists():
            config = cls()
            config._config_dir = str(config_path.expanduser().resolve().parent)
            return config
        with config_path.open("r", encoding="utf-8") as handle:
            raw = json.load(handle)
        known = {
            name
            for name, definition in cls.__dataclass_fields__.items()
            if definition.init
        }
        config = cls(**{key: value for key, value in raw.items() if key in known})
        config._config_dir = str(config_path.expanduser().resolve().parent)
        return config

    def validate(self) -> None:
        if self.runtime_mode not in {"embedded", "external"}:
            raise ValueError("runtime_mode must be embedded or external")
        if self.max_concurrent < 1:
            raise ValueError("max_concurrent must be at least 1")
        if self.task_queue_size < 1:
            raise ValueError("task_queue_size must be at least 1")
        if self.max_iterations < 1:
            raise ValueError("max_iterations must be at least 1")
        if self.cron_tick_seconds < 1:
            raise ValueError("cron_tick_seconds must be at least 1")

    def resolve_paths(self) -> None:
        config_dir = Path(self._config_dir).expanduser().resolve()
        workspace = self._resolve_path(self.workspace_dir, config_dir)
        self.workspace_dir = str(workspace)
        self.embedded_source = str(self._resolve_path(self.embedded_source, config_dir))
        self.hermes_source = str(self._resolve_path(self.hermes_source, config_dir))
        self.hermes_home = str(self._resolve_path(self.hermes_home, workspace))
        self.session_dir = str(self._resolve_path(self.session_dir, workspace))

    @staticmethod
    def _resolve_path(value: str, base: Path) -> Path:
        path = Path(value).expanduser()
        return path.resolve() if path.is_absolute() else (base / path).resolve()

    def prepare_native_home(self) -> None:
        home = Path(self.hermes_home)
        home.mkdir(parents=True, exist_ok=True)
        for name in ("cron", "logs", "memories", "scripts", "skills"):
            (home / name).mkdir(exist_ok=True)

        native = _deep_merge(
            {
                "model": {
                    "default": self.model,
                    "provider": self.provider or "auto",
                    "base_url": self.base_url,
                    "api_key": self.api_key,
                },
                "terminal": {"backend": "local", "cwd": self.workspace_dir},
            },
            self.native_config,
        )
        model = native.get("model")
        if isinstance(model, dict):
            native["model"] = {key: value for key, value in model.items() if value}
        target = home / "config.yaml"
        with target.open("w", encoding="utf-8", newline="\n") as handle:
            json.dump(native, handle, ensure_ascii=False, indent=2)
            handle.write("\n")
        os.chmod(target, 0o600)

    def write(self, path: str | Path) -> None:
        target = Path(path)
        target.parent.mkdir(parents=True, exist_ok=True)
        with target.open("w", encoding="utf-8", newline="\n") as handle:
            value = {
                key: item
                for key, item in self.__dict__.items()
                if not key.startswith("_")
            }
            json.dump(value, handle, ensure_ascii=False, indent=2)
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


def _deep_merge(base: dict[str, Any], overlay: dict[str, Any]) -> dict[str, Any]:
    result = dict(base)
    for key, value in overlay.items():
        if isinstance(value, dict) and isinstance(result.get(key), dict):
            result[key] = _deep_merge(result[key], value)
        else:
            result[key] = value
    return result
