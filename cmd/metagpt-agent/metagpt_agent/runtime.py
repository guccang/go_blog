from __future__ import annotations

import asyncio
import json
import logging
import time
import urllib.request
import uuid
from dataclasses import asdict, dataclass, field
from datetime import datetime
from typing import Any, Awaitable, Callable, Optional

from .config import AgentConfig, LLMConfig
from .session_store import SessionStore

logger = logging.getLogger(__name__)


def sanitize_tool_name(name: str) -> str:
    return name.replace(".", "_")


def unsanitize_tool_name(name: str) -> str:
    return name.replace("_", ".", 1) if "_" in name else name


def is_greeting(text: str) -> bool:
    normalized = (text or "").strip().lower()
    return normalized in {"hi", "hello", "hey", "你好", "您好", "哈喽", "在吗", "在嘛"}


def utcnow_iso() -> str:
    return datetime.utcnow().replace(microsecond=0).isoformat() + "Z"


@dataclass
class ToolCallSpec:
    id: str
    name: str
    arguments: str

    def to_openai(self) -> dict[str, Any]:
        return {
            "id": self.id,
            "type": "function",
            "function": {
                "name": self.name,
                "arguments": self.arguments,
            },
        }


@dataclass
class ChatMessage:
    role: str
    content: str = ""
    tool_calls: list[ToolCallSpec] = field(default_factory=list)
    tool_call_id: str = ""

    def to_openai(self) -> dict[str, Any]:
        payload = {
            "role": self.role,
            "content": self.content,
        }
        if self.tool_calls:
            payload["tool_calls"] = [call.to_openai() for call in self.tool_calls]
        if self.tool_call_id:
            payload["tool_call_id"] = self.tool_call_id
        return payload


@dataclass
class ToolDescriptor:
    agent_id: str
    original_name: str
    model_name: str
    description: str
    parameters: dict[str, Any]

    def to_openai_tool(self) -> dict[str, Any]:
        return {
            "type": "function",
            "function": {
                "name": self.model_name,
                "description": self.description,
                "parameters": self.parameters or {"type": "object", "properties": {}},
            },
        }


@dataclass
class AgentDescriptor:
    agent_id: str
    name: str
    description: str


@dataclass
class TaskRequest:
    task_id: str
    root_session_id: str
    task_type: str
    source_agent: str
    account: str
    source: str
    query: str = ""
    messages: list[ChatMessage] = field(default_factory=list)
    allowed_tools: list[str] = field(default_factory=list)
    no_tools: bool = False
    wechat_user: str = ""
    provider: str = ""
    model: str = ""
    direct_reply: bool = False


@dataclass
class TraceEvent:
    at: str
    kind: str
    detail: str
    data: dict[str, Any] = field(default_factory=dict)


@dataclass
class TaskExecutionResult:
    text: str
    status: str = "success"
    error: str = ""
    root_session_id: str = ""
    iterations: int = 0


class EventSink:
    async def on_event(self, event: str, text: str) -> None:
        return None

    async def on_chunk(self, text: str) -> None:
        await self.on_event("chunk", text)


class OpenAICompatibleLLMClient:
    def __init__(self, config: LLMConfig) -> None:
        self.config = config

    async def complete(
        self,
        config: LLMConfig,
        messages: list[ChatMessage],
        tools: list[ToolDescriptor],
        model_override: str = "",
    ) -> tuple[str, list[ToolCallSpec]]:
        return await asyncio.to_thread(
            self._complete_sync,
            config,
            messages,
            tools,
            model_override,
        )

    def _complete_sync(
        self,
        config: LLMConfig,
        messages: list[ChatMessage],
        tools: list[ToolDescriptor],
        model_override: str = "",
    ) -> tuple[str, list[ToolCallSpec]]:
        payload: dict[str, Any] = {
            "model": model_override or config.model,
            "messages": [message.to_openai() for message in messages],
            "max_tokens": config.max_tokens,
            "temperature": config.temperature,
        }
        if tools:
            payload["tools"] = [tool.to_openai_tool() for tool in tools]

        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        req = urllib.request.Request(
            url=f"{config.base_url.rstrip('/')}/chat/completions",
            data=body,
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {config.api_key}",
            },
            method="POST",
        )
        with urllib.request.urlopen(req, timeout=config.timeout_sec) as response:
            raw = response.read().decode("utf-8")
        data = json.loads(raw)
        choice = data["choices"][0]["message"]
        content = choice.get("content") or ""
        tool_calls = []
        for call in choice.get("tool_calls") or []:
            function = call.get("function") or {}
            tool_calls.append(
                ToolCallSpec(
                    id=call.get("id") or uuid.uuid4().hex,
                    name=function.get("name", ""),
                    arguments=function.get("arguments", "{}"),
                )
            )
        return content.strip(), tool_calls


ToolInvoker = Callable[[ToolCallSpec, str, EventSink], Awaitable[str]]


class QueryExecutor:
    def __init__(self, config: AgentConfig, store: SessionStore, llm: OpenAICompatibleLLMClient) -> None:
        self.config = config
        self.store = store
        self.llm = llm

    def build_system_prompt(self, request: TaskRequest, tools: list[ToolDescriptor]) -> str:
        now = datetime.now().strftime("%Y-%m-%d %H:%M")
        prompt = [
            self.config.system_prompt_prefix.strip(),
            f"当前账户: {request.account or 'unknown'}",
            f"当前来源: {request.source or 'unknown'}",
            f"当前时间: {now}",
            "工作规则:",
            "- 优先直接执行，不要空谈方案。",
            "- 只能基于真实工具结果回答。",
            "- 工具失败时说明失败原因，不要编造成功。",
        ]
        if tools:
            prompt.append("可用工具:")
            for tool in tools[:24]:
                prompt.append(f"- {tool.model_name}: {tool.description}")
        return "\n".join(prompt).strip()

    @staticmethod
    def infer_query(messages: list[ChatMessage]) -> str:
        for message in reversed(messages):
            if message.role == "user" and message.content.strip():
                return message.content.strip()
        return ""

    def select_tools(self, request: TaskRequest, available_tools: list[ToolDescriptor]) -> list[ToolDescriptor]:
        if request.no_tools or is_greeting(request.query):
            return []
        if not request.allowed_tools:
            return available_tools
        allowed = set(request.allowed_tools)
        selected = []
        for tool in available_tools:
            if tool.original_name in allowed or tool.model_name in allowed:
                selected.append(tool)
        return selected

    def _session_payload(
        self,
        request: TaskRequest,
        messages: list[ChatMessage],
        status: str,
        result: str = "",
        error: str = "",
    ) -> dict[str, Any]:
        return {
            "id": request.root_session_id,
            "root_id": request.root_session_id,
            "account": request.account,
            "source": request.source,
            "title": request.query,
            "status": status,
            "result": result,
            "error": error,
            "messages": [asdict(message) for message in messages],
            "updated_at": utcnow_iso(),
        }

    def _snapshot_payload(
        self,
        request: TaskRequest,
        messages: list[ChatMessage],
        tools: list[ToolDescriptor],
        status: str,
        iteration: int,
    ) -> dict[str, Any]:
        return {
            "root_id": request.root_session_id,
            "session_id": request.root_session_id,
            "query": request.query,
            "status": status,
            "iteration": iteration,
            "tool_names": [tool.model_name for tool in tools],
            "message_count": len(messages),
            "updated_at": utcnow_iso(),
        }

    async def execute(
        self,
        request: TaskRequest,
        available_tools: list[ToolDescriptor],
        sink: EventSink,
        tool_invoker: ToolInvoker,
    ) -> TaskExecutionResult:
        messages = list(request.messages)
        if not messages:
            system_prompt = self.build_system_prompt(request, available_tools)
            messages = [
                ChatMessage(role="system", content=system_prompt),
                ChatMessage(role="user", content=request.query),
            ]

        visible_tools = self.select_tools(request, available_tools)
        trace_events = [
            TraceEvent(
                at=utcnow_iso(),
                kind="start",
                detail="task started",
                data={
                    "task_type": request.task_type,
                    "query": request.query,
                    "tools": [tool.model_name for tool in visible_tools],
                },
            )
        ]
        self.store.save_session(
            request.root_session_id,
            request.root_session_id,
            self._session_payload(request, messages, "running"),
        )
        self.store.save_runtime_snapshot(
            request.root_session_id,
            request.root_session_id,
            self._snapshot_payload(request, messages, visible_tools, "running", 0),
        )

        iterations = 0
        for iterations in range(self.config.max_tool_iterations):
            text, tool_calls = await self.llm.complete(
                self.config.llm,
                messages,
                visible_tools,
                model_override=request.model,
            )
            assistant_message = ChatMessage(role="assistant", content=text, tool_calls=tool_calls)
            messages.append(assistant_message)
            trace_events.append(
                TraceEvent(
                    at=utcnow_iso(),
                    kind="assistant",
                    detail="assistant replied",
                    data={
                        "text_length": len(text),
                        "tool_calls": [call.name for call in tool_calls],
                    },
                )
            )
            self.store.save_session(
                request.root_session_id,
                request.root_session_id,
                self._session_payload(request, messages, "running"),
            )
            self.store.save_runtime_snapshot(
                request.root_session_id,
                request.root_session_id,
                self._snapshot_payload(request, messages, visible_tools, "running", iterations + 1),
            )

            if not tool_calls:
                final_text = text or "任务已完成。"
                self.store.save_session(
                    request.root_session_id,
                    request.root_session_id,
                    self._session_payload(request, messages, "done", result=final_text),
                )
                self.store.save_runtime_snapshot(
                    request.root_session_id,
                    request.root_session_id,
                    self._snapshot_payload(request, messages, visible_tools, "done", iterations + 1),
                )
                trace_events.append(
                    TraceEvent(
                        at=utcnow_iso(),
                        kind="finish",
                        detail="assistant returned final text",
                        data={"status": "success"},
                    )
                )
                self.store.save_trace(
                    request.root_session_id,
                    request.root_session_id,
                    {"events": [asdict(event) for event in trace_events]},
                )
                return TaskExecutionResult(
                    text=final_text,
                    status="success",
                    root_session_id=request.root_session_id,
                    iterations=iterations + 1,
                )

            for tool_call in tool_calls:
                await sink.on_event("tool_info", f"调用工具: {tool_call.name}")
                trace_events.append(
                    TraceEvent(
                        at=utcnow_iso(),
                        kind="tool_call",
                        detail=tool_call.name,
                        data={"arguments": tool_call.arguments},
                    )
                )
                result = await tool_invoker(tool_call, request.account, sink)
                messages.append(ChatMessage(role="tool", content=result, tool_call_id=tool_call.id))
                trace_events.append(
                    TraceEvent(
                        at=utcnow_iso(),
                        kind="tool_result",
                        detail=tool_call.name,
                        data={"result_preview": result[:400]},
                    )
                )
                self.store.save_session(
                    request.root_session_id,
                    request.root_session_id,
                    self._session_payload(request, messages, "running"),
                )
                self.store.save_runtime_snapshot(
                    request.root_session_id,
                    request.root_session_id,
                    self._snapshot_payload(request, messages, visible_tools, "running", iterations + 1),
                )

        fail_text = "达到最大工具迭代次数，未能在限制内完成任务。"
        messages.append(ChatMessage(role="assistant", content=fail_text))
        self.store.save_session(
            request.root_session_id,
            request.root_session_id,
            self._session_payload(request, messages, "failed", error=fail_text),
        )
        self.store.save_runtime_snapshot(
            request.root_session_id,
            request.root_session_id,
            self._snapshot_payload(request, messages, visible_tools, "failed", self.config.max_tool_iterations),
        )
        trace_events.append(
            TraceEvent(
                at=utcnow_iso(),
                kind="finish",
                detail="iteration limit reached",
                data={"status": "failed"},
            )
        )
        self.store.save_trace(
            request.root_session_id,
            request.root_session_id,
            {"events": [asdict(event) for event in trace_events]},
        )
        return TaskExecutionResult(
            text=fail_text,
            status="failed",
            error=fail_text,
            root_session_id=request.root_session_id,
            iterations=self.config.max_tool_iterations,
        )
