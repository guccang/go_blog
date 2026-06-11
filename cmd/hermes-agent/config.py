"""Hermes UAP agent configuration."""

from __future__ import annotations

import json
import os
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


_PROVIDER_ENV_VARS = {
    "deepseek": ("DEEPSEEK_API_KEY", "DEEPSEEK_BASE_URL"),
    "openai": ("OPENAI_API_KEY", "OPENAI_BASE_URL"),
    "openrouter": ("OPENROUTER_API_KEY", "OPENROUTER_BASE_URL"),
    "anthropic": ("ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL"),
    "gemini": ("GEMINI_API_KEY", "GEMINI_BASE_URL"),
    "google": ("GOOGLE_API_KEY", "GOOGLE_BASE_URL"),
    "xai": ("XAI_API_KEY", "XAI_BASE_URL"),
    "zai": ("ZAI_API_KEY", "ZAI_BASE_URL"),
    "kimi": ("KIMI_API_KEY", "KIMI_BASE_URL"),
    "minimax": ("MINIMAX_API_KEY", "MINIMAX_BASE_URL"),
}

_ENV_BEGIN = "# BEGIN go_blog hermes-agent managed model env"
_ENV_END = "# END go_blog hermes-agent managed model env"


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
    blog_agent_id: str = "blog-agent"
    gateway_http_url: str = ""
    tool_bridge_enabled: bool = True
    tool_call_timeout: float = 30.0
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
        self._install_go_blog_plugin(home)

        native = _deep_merge(
            {
                "terminal": {"backend": "local", "cwd": self.workspace_dir},
            },
            self.native_config,
        )
        if self.tool_bridge_enabled:
            plugins_cfg = native.get("plugins")
            if not isinstance(plugins_cfg, dict):
                plugins_cfg = {}
            enabled = plugins_cfg.get("enabled")
            if not isinstance(enabled, list):
                enabled = []
            if "go_blog" not in enabled:
                enabled = [*enabled, "go_blog"]
            plugins_cfg["enabled"] = enabled
            native["plugins"] = plugins_cfg
        model = native.get("model")
        if not isinstance(model, dict):
            model = {}
        for key, value in {
            "default": self.model,
            "provider": self.provider,
            "base_url": self.base_url,
            "api_key": self.api_key,
        }.items():
            if value:
                model[key] = value
        if not model.get("provider"):
            model["provider"] = "auto"
        native["model"] = {key: value for key, value in model.items() if value}
        self._sync_provider_env(home)
        target = home / "config.yaml"
        with target.open("w", encoding="utf-8", newline="\n") as handle:
            json.dump(native, handle, ensure_ascii=False, indent=2)
            handle.write("\n")
        os.chmod(target, 0o600)

    def _install_go_blog_plugin(self, home: Path) -> None:
        """把仓库内的 go_blog 插件同步到 hermes_home/plugins（用户插件目录）。"""
        if not self.tool_bridge_enabled:
            return
        source = Path(__file__).resolve().parent / "plugins" / "go_blog"
        if not source.is_dir():
            return
        target = home / "plugins" / "go_blog"
        target.mkdir(parents=True, exist_ok=True)
        for item in source.iterdir():
            if item.is_file():
                data = item.read_bytes()
                dest = target / item.name
                if not dest.exists() or dest.read_bytes() != data:
                    dest.write_bytes(data)

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

    def _sync_provider_env(self, home: Path) -> None:
        provider = self.provider.strip().lower()
        api_key = self.api_key.strip()
        base_url = self.base_url.strip()
        if not provider or (not api_key and not base_url):
            return
        key_env, base_url_env = _provider_env_names(provider)
        values: dict[str, str] = {}
        if api_key:
            values[key_env] = api_key
        if base_url:
            values[base_url_env] = base_url
        for key, value in values.items():
            os.environ[key] = value
        _write_managed_env_block(home / ".env", values)


def _deep_merge(base: dict[str, Any], overlay: dict[str, Any]) -> dict[str, Any]:
    result = dict(base)
    for key, value in overlay.items():
        if isinstance(value, dict) and isinstance(result.get(key), dict):
            result[key] = _deep_merge(result[key], value)
        else:
            result[key] = value
    return result


def _provider_env_names(provider: str) -> tuple[str, str]:
    names = _PROVIDER_ENV_VARS.get(provider)
    if names:
        return names
    normalized = "".join(ch if ch.isalnum() else "_" for ch in provider.upper())
    normalized = "_".join(part for part in normalized.split("_") if part)
    if not normalized:
        normalized = "HERMES_MODEL"
    return f"{normalized}_API_KEY", f"{normalized}_BASE_URL"


def _write_managed_env_block(path: Path, values: dict[str, str]) -> None:
    existing = ""
    if path.exists():
        existing = path.read_text(encoding="utf-8")
    kept_lines: list[str] = []
    in_block = False
    for line in existing.splitlines():
        if line == _ENV_BEGIN:
            in_block = True
            continue
        if line == _ENV_END:
            in_block = False
            continue
        if not in_block:
            kept_lines.append(line)

    block = [_ENV_BEGIN]
    for key in sorted(values):
        block.append(f"{key}={_env_value(values[key])}")
    block.append(_ENV_END)

    output_lines = [line for line in kept_lines if line.strip()]
    if output_lines:
        output_lines.append("")
    output_lines.extend(block)
    path.write_text("\n".join(output_lines) + "\n", encoding="utf-8", newline="\n")
    os.chmod(path, 0o600)


def _env_value(value: str) -> str:
    if any(ch in value for ch in "\r\n"):
        raise ValueError("environment value must not contain newlines")
    if not value or any(ch.isspace() or ch in "'\"#\\" for ch in value):
        escaped = value.replace("\\", "\\\\").replace('"', '\\"')
        return f'"{escaped}"'
    return value
