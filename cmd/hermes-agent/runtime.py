"""Hermes execution runtime and UAP message adaptation."""

from __future__ import annotations

import asyncio
import json
import logging
import os
import re
import sys
import threading
import uuid
from pathlib import Path
from typing import Any

from config import Config
from uap_client import UAPClient

logger = logging.getLogger(__name__)
APP_MESSAGE_PREFIX = "APP_MESSAGE_JSON:"
DELEGATION_PREFIX = re.compile(r"^\[delegation:[^\]]+\]")


def load_hermes_agent(config: Config) -> type:
    source = Path(config.hermes_source).expanduser().resolve()
    if not (source / "run_agent.py").exists():
        raise FileNotFoundError(f"Hermes source not found: {source}")
    sys.path.insert(0, str(source))
    from run_agent import AIAgent

    return AIAgent


def normalize_app_content(content: str) -> str:
    raw = DELEGATION_PREFIX.sub("", content.strip(), count=1).strip()
    if not raw.startswith(APP_MESSAGE_PREFIX):
        return raw
    body = raw[len(APP_MESSAGE_PREFIX) :].strip()
    try:
        message = json.loads(body)
    except json.JSONDecodeError:
        return raw
    sections = [str(message.get("content") or "").strip()]
    attachment = message.get("attachment") or {}
    speech = str(attachment.get("speech_text") or "").strip()
    if speech:
        sections.append(f"用户发送了语音，转写内容：{speech}")
    elif attachment:
        name = str(attachment.get("file_name") or "未命名附件")
        path = str(attachment.get("file_path") or "").strip()
        sections.append(f"用户发送了附件：{name}" + (f"\n本地路径：{path}" if path else ""))
    group_id = str(message.get("group_id") or "").strip()
    if group_id:
        sections.append(f"当前消息来自群聊 group_id={group_id}")
    return "\n\n".join(section for section in sections if section) or raw


def extract_task_query(payload: dict[str, Any]) -> tuple[str, str]:
    task_id = str(payload.get("task_id") or uuid.uuid4().hex)
    inner = payload.get("payload") or {}
    if isinstance(inner, str):
        try:
            inner = json.loads(inner)
        except json.JSONDecodeError:
            return task_id, inner
    for key in ("query", "content", "message", "prompt"):
        value = inner.get(key)
        if isinstance(value, str) and value.strip():
            return task_id, value.strip()
    messages = inner.get("messages") or []
    for message in reversed(messages):
        if message.get("role") == "user" and isinstance(message.get("content"), str):
            return task_id, message["content"].strip()
    return task_id, ""


def extract_task_account(payload: dict[str, Any], task_id: str) -> str:
    inner = payload.get("payload") or {}
    if isinstance(inner, str):
        try:
            inner = json.loads(inner)
        except json.JSONDecodeError:
            inner = {}
    account = str(inner.get("account") or "").strip()
    return account or f"task:{task_id}"


class SessionStore:
    def __init__(self, directory: str) -> None:
        self.directory = Path(directory)
        self.directory.mkdir(parents=True, exist_ok=True)
        self._locks: dict[str, threading.Lock] = {}
        self._guard = threading.Lock()

    def lock(self, key: str) -> threading.Lock:
        with self._guard:
            return self._locks.setdefault(key, threading.Lock())

    def load(self, key: str) -> list[dict[str, Any]]:
        path = self._path(key)
        if not path.exists():
            return []
        with path.open("r", encoding="utf-8") as handle:
            value = json.load(handle)
        return value if isinstance(value, list) else []

    def save(self, key: str, messages: list[dict[str, Any]]) -> None:
        path = self._path(key)
        temp = path.with_suffix(".tmp")
        with temp.open("w", encoding="utf-8", newline="\n") as handle:
            json.dump(messages, handle, ensure_ascii=False)
            handle.write("\n")
        os.replace(temp, path)

    def _path(self, key: str) -> Path:
        safe = re.sub(r"[^a-zA-Z0-9_.-]", "_", key)[:160] or "default"
        return self.directory / f"{safe}.json"


class HermesRuntime:
    def __init__(self, config: Config, agent_class: type | None = None) -> None:
        self.config = config
        self.agent_class = agent_class or load_hermes_agent(config)
        self.sessions = SessionStore(config.session_dir)
        self.loop = asyncio.get_running_loop()
        self.client: UAPClient | None = None
        self.queue: asyncio.Queue[tuple[str, str, str, str]] = asyncio.Queue(
            maxsize=config.task_queue_size
        )
        self.workers: list[asyncio.Task[None]] = []
        self._active_agents: dict[str, Any] = {}
        self._stopped_tasks: set[str] = set()
        self._active_guard = threading.Lock()

    def bind(self, client: UAPClient) -> None:
        self.client = client

    async def start(self) -> None:
        self.workers = [
            asyncio.create_task(self._worker(index))
            for index in range(self.config.max_concurrent)
        ]

    async def stop(self) -> None:
        for worker in self.workers:
            worker.cancel()
        await asyncio.gather(*self.workers, return_exceptions=True)

    async def handle_message(self, message: dict[str, Any]) -> None:
        message_type = message.get("type")
        payload = message.get("payload") or {}
        source = str(message.get("from") or "")
        if message_type == "notify" and payload.get("channel") == "app":
            await self._enqueue(
                source,
                str(payload.get("to") or "default"),
                normalize_app_content(str(payload.get("content") or "")),
                "",
            )
        elif message_type == "task_assign":
            task_id, query = extract_task_query(payload)
            await self._send("task_accepted", source, {"task_id": task_id})
            await self._enqueue(
                source, extract_task_account(payload, task_id), query, task_id
            )
        elif message_type == "task_stop":
            task_id = str(payload.get("task_id") or "")
            if task_id:
                self._stop_task(task_id)

    async def _enqueue(self, source: str, session_key: str, query: str, task_id: str) -> None:
        if not query:
            await self._deliver_error(source, session_key, task_id, "任务内容为空")
            return
        try:
            self.queue.put_nowait((source, session_key, query, task_id))
        except asyncio.QueueFull:
            await self._deliver_error(source, session_key, task_id, "Hermes Agent 任务队列已满")

    async def _worker(self, index: int) -> None:
        while True:
            source, session_key, query, task_id = await self.queue.get()
            try:
                if task_id and self._is_stopped(task_id):
                    await self._deliver_cancelled(source, task_id)
                    continue
                result = await asyncio.to_thread(
                    self._run_sync, source, session_key, query, task_id
                )
                if task_id and self._is_stopped(task_id):
                    await self._deliver_cancelled(source, task_id)
                else:
                    await self._deliver_result(source, session_key, task_id, result)
            except Exception as exc:
                logger.exception("worker %d failed task=%s", index, task_id or session_key)
                await self._deliver_error(source, session_key, task_id, str(exc))
            finally:
                self.queue.task_done()

    def _run_sync(self, source: str, session_key: str, query: str, task_id: str) -> str:
        with self.sessions.lock(session_key):
            history = self.sessions.load(session_key)

            def progress(text: Any, *_: Any, **__: Any) -> None:
                value = str(text or "").strip()
                if value:
                    self._schedule(self._send_progress(source, session_key, task_id, value))

            kwargs = self.config.agent_kwargs()
            kwargs.update(
                {
                    "session_id": session_key,
                    "user_id": session_key,
                    "chat_id": session_key,
                    "tool_progress_callback": progress,
                    "tool_start_callback": progress,
                    "status_callback": progress,
                }
            )
            agent = self.agent_class(**kwargs)
            if task_id:
                with self._active_guard:
                    self._active_agents[task_id] = agent
            try:
                result = agent.run_conversation(
                    query,
                    system_message=self.config.system_prompt,
                    conversation_history=history,
                    task_id=task_id or session_key,
                )
            finally:
                if task_id:
                    with self._active_guard:
                        self._active_agents.pop(task_id, None)
            messages = result.get("messages")
            if isinstance(messages, list):
                self.sessions.save(session_key, messages)
            response = str(result.get("final_response") or "").strip()
            return response or "Hermes 已完成处理，但没有返回文本结果。"

    def _schedule(self, coroutine: Any) -> None:
        self.loop.call_soon_threadsafe(asyncio.create_task, coroutine)

    async def _send_progress(
        self, source: str, session_key: str, task_id: str, content: str
    ) -> None:
        if task_id:
            await self._send(
                "task_event",
                source,
                {"task_id": task_id, "event": {"event": "tool_progress", "text": content}},
            )
        else:
            await self._send(
                "notify",
                source,
                {"channel": "app", "to": session_key, "content": content},
            )

    async def _deliver_result(
        self, source: str, session_key: str, task_id: str, result: str
    ) -> None:
        if task_id:
            await self._send(
                "task_complete",
                source,
                {"task_id": task_id, "status": "success", "result": result},
            )
        else:
            await self._send(
                "notify",
                source,
                {"channel": "app", "to": session_key, "content": result},
            )

    async def _deliver_error(
        self, source: str, session_key: str, task_id: str, error: str
    ) -> None:
        if task_id:
            await self._send(
                "task_complete",
                source,
                {"task_id": task_id, "status": "failed", "error": error},
            )
        else:
            await self._send(
                "notify",
                source,
                {"channel": "app", "to": session_key, "content": f"处理失败：{error}"},
            )

    async def _deliver_cancelled(self, source: str, task_id: str) -> None:
        await self._send(
            "task_complete",
            source,
            {"task_id": task_id, "status": "cancelled", "error": "任务已停止"},
        )
        with self._active_guard:
            self._stopped_tasks.discard(task_id)

    def _stop_task(self, task_id: str) -> None:
        with self._active_guard:
            self._stopped_tasks.add(task_id)
            agent = self._active_agents.get(task_id)
        if agent is not None:
            agent.interrupt("Stop requested")

    def _is_stopped(self, task_id: str) -> bool:
        with self._active_guard:
            return task_id in self._stopped_tasks

    async def _send(self, message_type: str, to: str, payload: dict[str, Any]) -> None:
        if self.client is None:
            raise RuntimeError("UAP client is not bound")
        await self.client.send(message_type, to=to, payload=payload)
