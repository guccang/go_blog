from __future__ import annotations

import json
from dataclasses import asdict, is_dataclass
from pathlib import Path
from typing import Any
from urllib.parse import quote_plus


def _to_jsonable(value: Any) -> Any:
    if is_dataclass(value):
        return {k: _to_jsonable(v) for k, v in asdict(value).items()}
    if isinstance(value, dict):
        return {str(k): _to_jsonable(v) for k, v in value.items()}
    if isinstance(value, list):
        return [_to_jsonable(v) for v in value]
    return value


class SessionStore:
    def __init__(self, session_dir: str, chat_session_dir: str) -> None:
        self.session_dir = Path(session_dir)
        self.chat_session_dir = Path(chat_session_dir)
        self.session_dir.mkdir(parents=True, exist_ok=True)
        self.chat_session_dir.mkdir(parents=True, exist_ok=True)

    def task_dir(self, root_id: str) -> Path:
        path = self.session_dir / root_id
        path.mkdir(parents=True, exist_ok=True)
        return path

    def save_task_file(self, root_id: str, name: str, payload: Any) -> None:
        path = self.task_dir(root_id) / name
        path.write_text(
            json.dumps(_to_jsonable(payload), ensure_ascii=False, indent=2),
            encoding="utf-8",
        )

    def load_task_file(self, root_id: str, name: str) -> dict[str, Any] | None:
        path = self.task_dir(root_id) / name
        if not path.exists():
            return None
        return json.loads(path.read_text(encoding="utf-8"))

    def save_session(self, root_id: str, session_id: str, payload: Any) -> None:
        self.save_task_file(root_id, f"session_{session_id}.json", payload)

    def load_session(self, root_id: str, session_id: str) -> dict[str, Any] | None:
        return self.load_task_file(root_id, f"session_{session_id}.json")

    def save_runtime_snapshot(self, root_id: str, session_id: str, payload: Any) -> None:
        self.save_task_file(root_id, f"runtime_{session_id}.json", payload)

    def load_runtime_snapshot(self, root_id: str, session_id: str) -> dict[str, Any] | None:
        return self.load_task_file(root_id, f"runtime_{session_id}.json")

    def save_trace(self, root_id: str, session_id: str, payload: Any) -> None:
        self.save_task_file(root_id, f"trace_{session_id}.json", payload)

    def _chat_path(self, source: str, user: str) -> Path:
        key = quote_plus(f"{source}:{user}")
        return self.chat_session_dir / f"chat_{key}.json"

    def save_chat_session(self, source: str, user: str, payload: Any) -> None:
        path = self._chat_path(source, user)
        path.write_text(
            json.dumps(_to_jsonable(payload), ensure_ascii=False, indent=2),
            encoding="utf-8",
        )

    def load_chat_session(self, source: str, user: str) -> dict[str, Any] | None:
        path = self._chat_path(source, user)
        if not path.exists():
            return None
        return json.loads(path.read_text(encoding="utf-8"))
