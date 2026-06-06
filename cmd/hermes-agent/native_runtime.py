"""Select and initialize the embedded or external Hermes runtime."""

from __future__ import annotations

import importlib
import os
import sys
from pathlib import Path
from typing import Any

from config import Config


def runtime_source(config: Config) -> Path:
    value = config.embedded_source if config.runtime_mode == "embedded" else config.hermes_source
    source = Path(value).expanduser().resolve()
    if not (source / "run_agent.py").exists():
        raise FileNotFoundError(f"Hermes {config.runtime_mode} runtime not found: {source}")
    return source


def initialize_native_runtime(config: Config) -> Path:
    source = runtime_source(config)
    os.environ["HERMES_HOME"] = config.hermes_home
    os.environ["HERMES_GATEWAY_SESSION"] = "1"
    os.environ["TERMINAL_CWD"] = config.workspace_dir
    if str(source) not in sys.path:
        sys.path.insert(0, str(source))
    config.prepare_native_home()
    return source


def load_agent_class(config: Config) -> type:
    initialize_native_runtime(config)
    module = importlib.import_module("run_agent")
    return module.AIAgent


def runtime_status(config: Config) -> dict[str, Any]:
    source = initialize_native_runtime(config)
    definitions: list[dict[str, Any]] = []
    try:
        agent_class = load_agent_class(config)
        model_tools = importlib.import_module("model_tools")
        definitions = model_tools.get_tool_definitions(quiet_mode=True)
    except Exception as exc:
        return {
            "mode": config.runtime_mode,
            "source": str(source),
            "hermes_home": config.hermes_home,
            "python": sys.executable,
            "ready": False,
            "error": str(exc),
        }
    names = [item.get("function", {}).get("name") for item in definitions]
    return {
        "mode": config.runtime_mode,
        "source": str(source),
        "hermes_home": config.hermes_home,
        "python": sys.executable,
        "run_agent": str(source / "run_agent.py"),
        "agent_class": agent_class.__name__,
        "tool_count": len(names),
        "cronjob": "cronjob" in names,
        "ready": True,
    }
