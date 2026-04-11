from __future__ import annotations

import asyncio
import json
import logging
import platform
import socket
import time
import urllib.request
from dataclasses import asdict
from typing import Any, Awaitable, Callable

import websockets

from .config import AgentConfig
from .protocol import (
    MessageEnvelope,
    NotifyPayload,
    RegisterPayload,
    TaskAcceptedPayload,
    TaskAssignPayload,
    TaskCompletePayload,
    TaskEventPayload,
    TaskRejectedPayload,
    ToolCallPayload,
    ToolDef,
    ToolResultPayload,
    json_dumps,
    new_msg_id,
)
from .role_runtime import MetaGPTTaskRole
from .runtime import (
    AgentDescriptor,
    ChatMessage,
    EventSink,
    OpenAICompatibleLLMClient,
    QueryExecutor,
    TaskExecutionResult,
    TaskRequest,
    ToolCallSpec,
    ToolDescriptor,
    unsanitize_tool_name,
)
from .session_store import SessionStore

logger = logging.getLogger(__name__)


class AsyncUAPClient:
    def __init__(
        self,
        config: AgentConfig,
        on_message: Callable[[MessageEnvelope], Awaitable[None]],
    ) -> None:
        self.config = config
        self.on_message = on_message
        self.websocket: websockets.WebSocketClientProtocol | None = None
        self.send_lock = asyncio.Lock()

    async def run_forever(self, tools: list[ToolDef], meta: dict[str, Any]) -> None:
        backoff = 1
        while True:
            try:
                async with websockets.connect(self.config.gateway_url, max_size=None, ping_interval=None) as websocket:
                    self.websocket = websocket
                    await self._register(tools, meta)
                    heartbeat_task = asyncio.create_task(self._heartbeat_loop())
                    try:
                        async for raw in websocket:
                            await self.on_message(MessageEnvelope.from_wire(raw))
                    finally:
                        heartbeat_task.cancel()
                        self.websocket = None
                backoff = 1
            except Exception as exc:
                logger.warning("gateway connection failed: %s", exc)
                await asyncio.sleep(backoff)
                backoff = min(backoff * 2, 15)

    async def _register(self, tools: list[ToolDef], meta: dict[str, Any]) -> None:
        payload = RegisterPayload(
            agent_id=self.config.agent_id,
            agent_type=self.config.agent_type,
            name=self.config.agent_name,
            description=self.config.description,
            host_platform=platform.system().lower(),
            host_ip=self._host_ip(),
            workspace=self.config.workspace_dir,
            tools=[tool.to_dict() for tool in tools],
            capacity=self.config.max_concurrent,
            meta=meta,
            auth_token=self.config.auth_token,
        )
        await self.send(
            MessageEnvelope(
                type="register",
                id=new_msg_id(),
                from_id=self.config.agent_id,
                payload=payload.to_dict(),
            )
        )

    async def _heartbeat_loop(self) -> None:
        while self.websocket is not None:
            await asyncio.sleep(self.config.heartbeat_interval_sec)
            if self.websocket is None:
                return
            await self.send(
                MessageEnvelope(
                    type="heartbeat",
                    id=new_msg_id(),
                    from_id=self.config.agent_id,
                    payload={"agent_id": self.config.agent_id},
                )
            )

    async def send(self, envelope: MessageEnvelope) -> None:
        if self.websocket is None:
            raise RuntimeError("gateway websocket is not connected")
        async with self.send_lock:
            await self.websocket.send(envelope.to_wire())

    @staticmethod
    def _host_ip() -> str:
        try:
            return socket.gethostbyname(socket.gethostname())
        except OSError:
            return "127.0.0.1"


class TaskEventSink(EventSink):
    def __init__(self, service: "MetaGPTAgentService", task_id: str, target_agent: str) -> None:
        self.service = service
        self.task_id = task_id
        self.target_agent = target_agent

    async def on_event(self, event: str, text: str) -> None:
        await self.service.send_task_event(self.task_id, self.target_agent, event, text)


class NullSink(EventSink):
    async def on_event(self, event: str, text: str) -> None:
        logger.info("sink[%s] %s", event, text)


class MetaGPTAgentService:
    def __init__(self, config: AgentConfig) -> None:
        self.config = config
        self.config.ensure_dirs()
        self.store = SessionStore(config.session_dir, config.chat_session_dir)
        self.llm_client = OpenAICompatibleLLMClient(config.llm)
        self.executor = QueryExecutor(config, self.store, self.llm_client)
        self.client = AsyncUAPClient(config, self.handle_message)
        self.pending_tool_results: dict[str, asyncio.Future[ToolResultPayload]] = {}
        self.progress_sinks: dict[str, EventSink] = {}
        self.tool_catalog: dict[str, ToolDescriptor] = {}
        self.agents: dict[str, AgentDescriptor] = {}
        self.queue: asyncio.Queue[Callable[[], Awaitable[None]]] = asyncio.Queue(maxsize=config.task_queue_size)
        self.workers: list[asyncio.Task[Any]] = []
        self.bg_tasks: list[asyncio.Task[Any]] = []

    async def start(self) -> None:
        await self.refresh_gateway_state()
        self.workers = [
            asyncio.create_task(self.worker_loop(idx))
            for idx in range(self.config.max_concurrent)
        ]
        self.bg_tasks = [asyncio.create_task(self.discovery_loop())]
        await self.client.run_forever(
            tools=[],
            meta={
                "framework": "MetaGPT",
                "python": platform.python_version(),
                "supports_notify": True,
                "supports_tasks": [
                    "assistant_chat",
                    "llm_request",
                    "resume_task",
                    "cron_reminder",
                    "cron_query",
                ],
            },
        )

    async def discovery_loop(self) -> None:
        while True:
            await asyncio.sleep(self.config.discovery_interval_sec)
            try:
                await self.refresh_gateway_state()
            except Exception as exc:
                logger.warning("refresh gateway state failed: %s", exc)

    async def worker_loop(self, idx: int) -> None:
        while True:
            job = await self.queue.get()
            try:
                await job()
            except Exception as exc:
                logger.exception("worker %s job failed: %s", idx, exc)
            finally:
                self.queue.task_done()

    async def handle_message(self, message: MessageEnvelope) -> None:
        msg_type = message.type
        if msg_type == "tool_result":
            payload = ToolResultPayload.from_dict(message.payload)
            future = self.pending_tool_results.pop(payload.request_id, None)
            if future and not future.done():
                future.set_result(payload)
            return

        if msg_type == "error":
            future = self.pending_tool_results.pop(message.id, None)
            if future and not future.done():
                future.set_exception(RuntimeError(message.payload.get("message", "unknown gateway error")))
            return

        if msg_type == "notify":
            await self.handle_notify(message)
            return

        if msg_type != "task_assign":
            return

        payload = TaskAssignPayload(
            task_id=message.payload.get("task_id", ""),
            payload=message.payload.get("payload", {}),
        )
        await self.handle_task_assign(message.from_id, payload)

    async def handle_notify(self, message: MessageEnvelope) -> None:
        payload = message.payload or {}
        channel = (payload.get("channel") or "").strip()
        if channel == "tool_progress":
            sink = self.progress_sinks.get((payload.get("to") or "").strip())
            if sink:
                await sink.on_event("tool_progress", payload.get("content", ""))
            return
        if channel not in {"wechat", "app"}:
            return

        from_agent = message.from_id
        to_user = (payload.get("to") or "").strip()
        content = payload.get("content", "") or ""

        async def job() -> None:
            await self.process_direct_chat(channel, from_agent, to_user, content)

        await self.enqueue(job)

    async def handle_task_assign(self, source_agent: str, payload: TaskAssignPayload) -> None:
        body = payload.payload or {}
        task_type = body.get("task_type", "")

        async def job() -> None:
            if task_type == "assistant_chat":
                await self.process_assistant_chat(source_agent, payload.task_id, body)
            elif task_type == "llm_request":
                await self.process_llm_request(source_agent, payload.task_id, body)
            elif task_type == "resume_task":
                await self.process_resume_task(source_agent, payload.task_id, body)
            elif task_type == "cron_reminder":
                await self.process_cron_reminder(source_agent, payload.task_id, body)
            elif task_type == "cron_query":
                await self.process_cron_query(source_agent, payload.task_id, body)
            else:
                await self.send_task_complete(
                    payload.task_id,
                    source_agent,
                    status="failed",
                    error=f"unknown task_type: {task_type}",
                )

        try:
            self.queue.put_nowait(job)
        except asyncio.QueueFull:
            await self.send_message(
                MessageEnvelope(
                    type="task_rejected",
                    id=new_msg_id(),
                    from_id=self.config.agent_id,
                    to=source_agent,
                    payload=asdict(
                        TaskRejectedPayload(
                            task_id=payload.task_id,
                            reason="task queue is full",
                        )
                    ),
                )
            )
            return

        await self.send_message(
            MessageEnvelope(
                type="task_accepted",
                id=new_msg_id(),
                from_id=self.config.agent_id,
                to=source_agent,
                payload=asdict(TaskAcceptedPayload(task_id=payload.task_id)),
            )
        )

    async def enqueue(self, job: Callable[[], Awaitable[None]]) -> None:
        try:
            self.queue.put_nowait(job)
        except asyncio.QueueFull:
            logger.warning("direct chat queue is full, dropping task")

    async def send_message(self, envelope: MessageEnvelope) -> None:
        await self.client.send(envelope)

    async def send_task_event(self, task_id: str, target_agent: str, event: str, text: str) -> None:
        await self.send_message(
            MessageEnvelope(
                type="task_event",
                id=new_msg_id(),
                from_id=self.config.agent_id,
                to=target_agent,
                payload=asdict(
                    TaskEventPayload(
                        task_id=task_id,
                        event={"event": event, "text": text},
                    )
                ),
            )
        )

    async def send_task_complete(
        self,
        task_id: str,
        target_agent: str,
        status: str,
        error: str = "",
        result: str = "",
    ) -> None:
        await self.send_message(
            MessageEnvelope(
                type="task_complete",
                id=new_msg_id(),
                from_id=self.config.agent_id,
                to=target_agent,
                payload=asdict(
                    TaskCompletePayload(
                        task_id=task_id,
                        status=status,
                        error=error,
                        result=result,
                    )
                ),
            )
        )

    async def refresh_gateway_state(self) -> None:
        self.tool_catalog = await asyncio.to_thread(self.fetch_tools)
        self.agents = await asyncio.to_thread(self.fetch_agents)
        logger.info("gateway state refreshed: tools=%d agents=%d", len(self.tool_catalog), len(self.agents))

    def fetch_tools(self) -> dict[str, ToolDescriptor]:
        url = f"{self.config.gateway_http.rstrip('/')}/api/gateway/tools"
        with urllib.request.urlopen(url, timeout=15) as response:
            data = json.loads(response.read().decode("utf-8"))
        result: dict[str, ToolDescriptor] = {}
        for item in data.get("tools", []):
            original_name = item.get("name", "")
            descriptor = ToolDescriptor(
                agent_id=item.get("agent_id", ""),
                original_name=original_name,
                model_name=original_name.replace(".", "_"),
                description=item.get("description", "") or original_name,
                parameters=item.get("parameters") or {"type": "object", "properties": {}},
            )
            result[descriptor.original_name] = descriptor
        return result

    def fetch_agents(self) -> dict[str, AgentDescriptor]:
        url = f"{self.config.gateway_http.rstrip('/')}/api/gateway/agents"
        with urllib.request.urlopen(url, timeout=15) as response:
            data = json.loads(response.read().decode("utf-8"))
        result: dict[str, AgentDescriptor] = {}
        for item in data.get("agents", []):
            descriptor = AgentDescriptor(
                agent_id=item.get("agent_id", ""),
                name=item.get("name", ""),
                description=item.get("description", ""),
            )
            result[descriptor.agent_id] = descriptor
        return result

    def visible_tools(self, request: TaskRequest) -> list[ToolDescriptor]:
        tools = list(self.tool_catalog.values())
        if not request.allowed_tools:
            return tools
        allowed = set(request.allowed_tools)
        return [
            tool
            for tool in tools
            if tool.original_name in allowed or tool.model_name in allowed
        ]

    def build_task_request(
        self,
        task_id: str,
        source_agent: str,
        task_type: str,
        source: str,
        account: str,
        query: str = "",
        messages: list[dict[str, Any]] | None = None,
        allowed_tools: list[str] | None = None,
        no_tools: bool = False,
        wechat_user: str = "",
        model: str = "",
        root_session_id: str = "",
    ) -> TaskRequest:
        parsed_messages = [
            ChatMessage(
                role=item.get("role", "user"),
                content=item.get("content", "") or "",
                tool_calls=[
                    ToolCallSpec(
                        id=call.get("id", ""),
                        name=((call.get("function") or {}).get("name", "")),
                        arguments=((call.get("function") or {}).get("arguments", "{}")),
                    )
                    for call in item.get("tool_calls") or []
                ],
                tool_call_id=item.get("tool_call_id", "") or "",
            )
            for item in (messages or [])
        ]
        if not query:
            query = self.executor.infer_query(parsed_messages)
        return TaskRequest(
            task_id=task_id,
            root_session_id=root_session_id or task_id,
            task_type=task_type,
            source_agent=source_agent,
            account=account,
            source=source,
            query=query,
            messages=parsed_messages,
            allowed_tools=allowed_tools or [],
            no_tools=no_tools,
            wechat_user=wechat_user,
            model=model,
        )

    async def run_metagpt_task(self, request: TaskRequest, sink: EventSink) -> TaskExecutionResult:
        role = MetaGPTTaskRole(self.config, self.executor)
        return await role.run_request(
            request,
            self.visible_tools(request),
            sink,
            self.call_tool,
        )

    async def call_tool(self, tool_call: ToolCallSpec, account: str, sink: EventSink) -> str:
        model_name = tool_call.name
        original_name = unsanitize_tool_name(model_name)
        descriptor = self.tool_catalog.get(original_name)
        if descriptor is None:
            for candidate in self.tool_catalog.values():
                if candidate.model_name == model_name:
                    descriptor = candidate
                    break
        if descriptor is None:
            return f"工具不可用: {model_name}"

        try:
            arguments = json.loads(tool_call.arguments or "{}")
        except json.JSONDecodeError as exc:
            return f"工具参数解析失败: {exc}"

        request_id = new_msg_id()
        future: asyncio.Future[ToolResultPayload] = asyncio.get_running_loop().create_future()
        self.pending_tool_results[request_id] = future
        self.progress_sinks[request_id] = sink
        try:
            await self.send_message(
                MessageEnvelope(
                    type="tool_call",
                    id=request_id,
                    from_id=self.config.agent_id,
                    to=descriptor.agent_id,
                    payload=ToolCallPayload(
                        tool_name=descriptor.original_name,
                        arguments=arguments,
                        authenticated_user=account,
                    ).to_dict(),
                )
            )
            payload = await asyncio.wait_for(future, timeout=self.config.tool_call_timeout_sec)
        except asyncio.TimeoutError:
            self.pending_tool_results.pop(request_id, None)
            return f"工具调用超时: {descriptor.original_name}"
        finally:
            self.progress_sinks.pop(request_id, None)

        if not payload.success:
            return f"工具调用失败: {payload.error or payload.result}"
        return payload.result or ""

    async def process_assistant_chat(self, source_agent: str, task_id: str, body: dict[str, Any]) -> None:
        request = self.build_task_request(
            task_id=task_id,
            source_agent=source_agent,
            task_type="assistant_chat",
            source="web",
            account=(body.get("account") or "").strip(),
            query=body.get("query", "") or "",
        )
        sink = TaskEventSink(self, task_id, source_agent)
        result = await self.run_metagpt_task(request, sink)
        await sink.on_chunk(result.text)
        await self.send_task_complete(task_id, source_agent, result.status, result.error)

    async def process_llm_request(self, source_agent: str, task_id: str, body: dict[str, Any]) -> None:
        request = self.build_task_request(
            task_id=task_id,
            source_agent=source_agent,
            task_type="llm_request",
            source="llm_request",
            account=(body.get("account") or "").strip(),
            messages=body.get("messages") or [],
            allowed_tools=body.get("allowed_tools") or [],
            no_tools=bool(body.get("no_tools")),
        )
        result = await self.run_metagpt_task(request, NullSink())
        await self.send_task_complete(task_id, source_agent, result.status, result.error, result.text)

    async def process_resume_task(self, source_agent: str, task_id: str, body: dict[str, Any]) -> None:
        root_session_id = (body.get("root_session_id") or "").strip()
        if not root_session_id:
            await self.send_task_complete(task_id, source_agent, "failed", "missing root_session_id")
            return
        session = self.store.load_session(root_session_id, root_session_id)
        if not session:
            await self.send_task_complete(task_id, source_agent, "failed", f"session not found: {root_session_id}")
            return
        request = self.build_task_request(
            task_id=task_id,
            source_agent=source_agent,
            task_type="resume_task",
            source=session.get("source", "resume_task"),
            account=(session.get("account") or "").strip(),
            query=session.get("title", "") or "",
            messages=session.get("messages") or [],
            root_session_id=root_session_id,
        )
        sink = TaskEventSink(self, task_id, source_agent)
        result = await self.run_metagpt_task(request, sink)
        await sink.on_chunk(result.text)
        await self.send_task_complete(task_id, source_agent, result.status, result.error, result.text)

    async def process_cron_reminder(self, source_agent: str, task_id: str, body: dict[str, Any]) -> None:
        payload = body.get("payload") or {}
        account = (payload.get("account") or "").strip()
        wechat_user = (payload.get("wechat_user") or "").strip()
        message = payload.get("message", "") or ""
        error = await self.send_cron_notification(account, wechat_user, "⏰ " + message)
        status = "failed" if error else "success"
        await self.send_task_complete(task_id, source_agent, status, error or "")

    async def process_cron_query(self, source_agent: str, task_id: str, body: dict[str, Any]) -> None:
        payload = body.get("payload") or {}
        account = (payload.get("account") or "").strip()
        wechat_user = (payload.get("wechat_user") or "").strip()
        query = payload.get("query", "") or ""
        request = self.build_task_request(
            task_id=task_id,
            source_agent=source_agent,
            task_type="cron_query",
            source="cron_query",
            account=wechat_user or account,
            query=query,
            wechat_user=wechat_user,
            model=body.get("model", "") or "",
        )
        result = await self.run_metagpt_task(request, NullSink())
        error = await self.send_cron_notification(account, wechat_user, result.text) if result.text else ""
        if result.error:
            error = f"{result.error}; {error}" if error else result.error
        status = "failed" if error or result.status != "success" else "success"
        await self.send_task_complete(task_id, source_agent, status, error, result.text)

    async def process_direct_chat(self, channel: str, source_agent: str, user: str, content: str) -> None:
        saved = self.store.load_chat_session(channel, user) or {}
        messages = saved.get("messages") or []
        if not messages:
            system_prompt = self.executor.build_system_prompt(
                TaskRequest(
                    task_id="bootstrap",
                    root_session_id="bootstrap",
                    task_type=f"{channel}_chat",
                    source_agent=source_agent,
                    account=user,
                    source=channel,
                    query=content,
                ),
                self.visible_tools(
                    TaskRequest(
                        task_id="bootstrap",
                        root_session_id="bootstrap",
                        task_type=f"{channel}_chat",
                        source_agent=source_agent,
                        account=user,
                        source=channel,
                        query=content,
                    )
                ),
            )
            messages = [{"role": "system", "content": system_prompt}]
        messages.append({"role": "user", "content": content})

        request = self.build_task_request(
            task_id=f"{channel}_{int(time.time() * 1000)}",
            source_agent=source_agent,
            task_type=f"{channel}_chat",
            source=channel,
            account=user,
            query=content,
            messages=messages,
        )
        result = await self.run_metagpt_task(request, NullSink())
        messages.append({"role": "assistant", "content": result.text})
        self.store.save_chat_session(
            channel,
            user,
            {
                "source": channel,
                "user": user,
                "messages": messages[-40:],
                "updated_at": int(time.time()),
            },
        )
        await self.send_message(
            MessageEnvelope(
                type="notify",
                id=new_msg_id(),
                from_id=self.config.agent_id,
                to=source_agent,
                payload=NotifyPayload(channel=channel, to=user, content=result.text).to_dict(),
            )
        )

    async def send_cron_notification(self, account: str, wechat_user: str, content: str) -> str:
        errors: list[str] = []
        wechat_agent = self.find_agent_id("wechat")
        app_agent = self.find_app_agent_id()
        if wechat_user:
            if not wechat_agent:
                errors.append("no wechat-agent online")
            else:
                await self.send_message(
                    MessageEnvelope(
                        type="notify",
                        id=new_msg_id(),
                        from_id=self.config.agent_id,
                        to=wechat_agent,
                        payload=NotifyPayload(channel="wechat", to=wechat_user, content=content).to_dict(),
                    )
                )
        app_user = account or wechat_user
        if app_user:
            if not app_agent:
                errors.append("no app-agent online")
            else:
                await self.send_message(
                    MessageEnvelope(
                        type="notify",
                        id=new_msg_id(),
                        from_id=self.config.agent_id,
                        to=app_agent,
                        payload=NotifyPayload(channel="app", to=app_user, content=content).to_dict(),
                    )
                )
        return "; ".join(errors)

    def find_agent_id(self, keyword: str) -> str:
        needle = keyword.lower()
        for agent in self.agents.values():
            agent_id = agent.agent_id.lower()
            agent_name = agent.name.lower()
            if needle in agent_id or needle in agent_name:
                return agent.agent_id
        return ""

    def find_app_agent_id(self) -> str:
        for agent in self.agents.values():
            agent_id = agent.agent_id.lower()
            agent_name = agent.name.lower()
            if "app-agent" in agent_id or "app-agent" in agent_name or agent_id.startswith("app-"):
                return agent.agent_id
        return ""
