"""Minimal UAP WebSocket client used by Hermes Agent."""

from __future__ import annotations

import asyncio
import json
import logging
import platform
import time
import uuid
from collections.abc import Awaitable, Callable
from typing import Any

import aiohttp

logger = logging.getLogger(__name__)
MessageHandler = Callable[[dict[str, Any]], Awaitable[None]]


def new_message(
    message_type: str,
    agent_id: str,
    *,
    to: str = "",
    payload: dict[str, Any] | None = None,
    message_id: str | None = None,
) -> dict[str, Any]:
    return {
        "type": message_type,
        "id": message_id or uuid.uuid4().hex,
        "from": agent_id,
        "to": to,
        "payload": payload or {},
        "ts": int(time.time() * 1000),
    }


class UAPClient:
    def __init__(
        self,
        gateway_url: str,
        agent_id: str,
        agent_name: str,
        auth_token: str,
        capacity: int,
        workspace: str,
        handler: MessageHandler,
    ) -> None:
        self.gateway_url = gateway_url
        self.agent_id = agent_id
        self.agent_name = agent_name
        self.auth_token = auth_token
        self.capacity = capacity
        self.workspace = workspace
        self.handler = handler
        self._ws: aiohttp.ClientWebSocketResponse | None = None
        self._send_lock = asyncio.Lock()
        self._stopping = asyncio.Event()

    async def run(self) -> None:
        backoff = 1
        async with aiohttp.ClientSession() as session:
            while not self._stopping.is_set():
                try:
                    logger.info("connecting to %s", self.gateway_url)
                    async with session.ws_connect(self.gateway_url, heartbeat=30) as ws:
                        self._ws = ws
                        backoff = 1
                        await self._register()
                        heartbeat = asyncio.create_task(self._heartbeat_loop())
                        try:
                            async for incoming in ws:
                                if incoming.type == aiohttp.WSMsgType.TEXT:
                                    await self.handler(json.loads(incoming.data))
                                elif incoming.type in {
                                    aiohttp.WSMsgType.CLOSED,
                                    aiohttp.WSMsgType.ERROR,
                                }:
                                    break
                        finally:
                            heartbeat.cancel()
                            await asyncio.gather(heartbeat, return_exceptions=True)
                except asyncio.CancelledError:
                    raise
                except Exception:
                    logger.exception("gateway connection failed")
                finally:
                    self._ws = None
                if not self._stopping.is_set():
                    await asyncio.sleep(backoff)
                    backoff = min(backoff * 2, 30)

    async def stop(self) -> None:
        self._stopping.set()
        if self._ws is not None:
            await self._ws.close()

    async def send(
        self,
        message_type: str,
        *,
        to: str = "",
        payload: dict[str, Any] | None = None,
        message_id: str | None = None,
    ) -> None:
        ws = self._ws
        if ws is None or ws.closed:
            raise RuntimeError("gateway is not connected")
        message = new_message(
            message_type,
            self.agent_id,
            to=to,
            payload=payload,
            message_id=message_id,
        )
        async with self._send_lock:
            await ws.send_json(message, dumps=lambda value: json.dumps(value, ensure_ascii=False))

    async def _register(self) -> None:
        await self.send(
            "register",
            payload={
                "agent_id": self.agent_id,
                "agent_type": "llm_mcp",
                "name": self.agent_name,
                "description": "Hermes Agent runtime for Flutter and UAP tasks",
                "host_platform": platform.system(),
                "host_ip": "",
                "workspace": self.workspace,
                "tools": [],
                "capacity": self.capacity,
                "meta": {
                    "runtime": "hermes",
                    "supports_app": True,
                    "supports_task_types": ["assistant_chat", "llm_request"],
                },
                "auth_token": self.auth_token,
            },
        )

    async def _heartbeat_loop(self) -> None:
        while not self._stopping.is_set():
            await asyncio.sleep(15)
            await self.send("heartbeat", payload={"agent_id": self.agent_id, "load": 0})
