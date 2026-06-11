"""Bridge blog-agent MCP tools into the Hermes tool registry via UAP tool_call."""

from __future__ import annotations

import asyncio
import json
import logging
import threading
import urllib.request
from typing import Any
from urllib.parse import urlsplit, urlunsplit

logger = logging.getLogger(__name__)

GO_BLOG_TOOLSET = "go_blog"
_DISCOVER_TIMEOUT = 5.0


def gateway_http_url(gateway_url: str) -> str:
    """Derive the gateway HTTP base URL from its websocket URL."""
    parts = urlsplit(gateway_url)
    scheme = "https" if parts.scheme == "wss" else "http"
    return urlunsplit((scheme, parts.netloc, "", "", ""))


def sanitize_tool_name(name: str) -> str:
    return name.replace(".", "_")


class GoBlogToolBridge:
    """Process-wide bridge between Hermes tool handlers and the UAP client.

    Hermes 工具 handler 在 agent 工作线程中同步执行，而 UAP websocket
    在主 asyncio loop 上；这里用 run_coroutine_threadsafe 跨线程桥接。
    """

    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._config: Any = None
        self._client: Any = None
        self._loop: asyncio.AbstractEventLoop | None = None
        # 原始工具名映射：注册名(下划线) -> blog-agent 原始名
        self._raw_names: dict[str, str] = {}

    def configure(self, config: Any, client: Any, loop: asyncio.AbstractEventLoop) -> None:
        with self._lock:
            self._config = config
            self._client = client
            self._loop = loop

    @property
    def ready(self) -> bool:
        with self._lock:
            return self._client is not None and self._loop is not None

    @property
    def config(self) -> Any:
        with self._lock:
            return self._config

    def remember_raw_name(self, registered: str, raw: str) -> None:
        with self._lock:
            self._raw_names[registered] = raw

    def discover_blog_tools(
        self, http_base: str, blog_agent_id: str
    ) -> list[dict[str, Any]]:
        """Fetch blog-agent tool definitions from the gateway registry."""
        url = f"{http_base}/api/gateway/tools"
        with urllib.request.urlopen(url, timeout=_DISCOVER_TIMEOUT) as response:
            body = json.loads(response.read().decode("utf-8"))
        if not body.get("success"):
            raise RuntimeError("gateway returned success=false")
        tools: list[dict[str, Any]] = []
        for item in body.get("tools") or []:
            if not isinstance(item, dict):
                continue
            if str(item.get("agent_id") or "") != blog_agent_id:
                continue
            name = str(item.get("name") or "").strip()
            if not name:
                continue
            parameters = item.get("parameters")
            if not isinstance(parameters, dict) or not parameters:
                parameters = {"type": "object", "properties": {}}
            tools.append(
                {
                    "name": name,
                    "description": str(item.get("description") or ""),
                    "parameters": parameters,
                }
            )
        return tools

    def call(self, registered_name: str, args: dict[str, Any]) -> str:
        from tools.registry import tool_error

        with self._lock:
            client = self._client
            loop = self._loop
            config = self._config
            raw_name = self._raw_names.get(registered_name, registered_name)
        if client is None or loop is None or config is None:
            return tool_error("go_blog 工具桥尚未就绪（UAP 未连接）")

        arguments = dict(args or {})
        authenticated_user = self._current_user()
        # blog-agent 工具普遍以 account 区分用户数据；未显式提供时注入当前会话用户
        if authenticated_user and not str(arguments.get("account") or "").strip():
            arguments["account"] = authenticated_user

        timeout = float(getattr(config, "tool_call_timeout", 30.0))
        future = asyncio.run_coroutine_threadsafe(
            client.call_tool(
                config.blog_agent_id,
                raw_name,
                arguments,
                authenticated_user=authenticated_user,
                timeout=timeout,
            ),
            loop,
        )
        try:
            payload = future.result(timeout=timeout + 5.0)
        except Exception as exc:
            future.cancel()
            return tool_error(f"调用 blog-agent 工具 {raw_name} 失败: {exc}")
        if payload.get("success"):
            result = payload.get("result")
            return result if isinstance(result, str) else json.dumps(result, ensure_ascii=False)
        return tool_error(str(payload.get("error") or "blog-agent 返回失败"))

    @staticmethod
    def _current_user() -> str:
        try:
            from gateway.session_context import get_session_env

            return get_session_env("HERMES_SESSION_USER_ID").strip()
        except Exception:
            return ""


bridge = GoBlogToolBridge()


def register_plugin_tools(ctx: Any) -> None:
    """Entry used by the go_blog Hermes plugin: discover and register blog tools."""
    config = bridge.config
    if config is None:
        logger.warning("go_blog plugin loaded before bridge configured; skip registration")
        return
    if not getattr(config, "tool_bridge_enabled", True):
        return

    http_base = getattr(config, "gateway_http_url", "") or gateway_http_url(config.gateway_url)
    try:
        tools = bridge.discover_blog_tools(http_base, config.blog_agent_id)
    except Exception as exc:
        logger.warning("go_blog tool discovery failed (%s): %s", http_base, exc)
        return

    registered = 0
    for tool in tools:
        raw_name = tool["name"]
        registered_name = sanitize_tool_name(raw_name)
        bridge.remember_raw_name(registered_name, raw_name)

        def make_handler(name: str):
            def handler(args: dict, **_: Any) -> str:
                return bridge.call(name, args)

            return handler

        try:
            ctx.register_tool(
                name=registered_name,
                toolset=GO_BLOG_TOOLSET,
                schema={
                    "name": registered_name,
                    "description": tool["description"],
                    "parameters": tool["parameters"],
                },
                handler=make_handler(registered_name),
                description=tool["description"],
                emoji="📝",
            )
            registered += 1
        except Exception as exc:
            logger.warning("register go_blog tool %s failed: %s", registered_name, exc)
    logger.info("go_blog plugin registered %d blog-agent tools", registered)
