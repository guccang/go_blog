"""Run native Hermes cron jobs and route App-origin results through UAP."""

from __future__ import annotations

import asyncio
import logging
from typing import Any

from config import Config
from uap_client import UAPClient

logger = logging.getLogger(__name__)


class CronBridge:
    def __init__(self, config: Config, client: UAPClient, loop: asyncio.AbstractEventLoop) -> None:
        self.config = config
        self.client = client
        self.loop = loop
        self._task: asyncio.Task[None] | None = None
        self._scheduler: Any = None
        self._original_deliver: Any = None

    async def start(self) -> None:
        from cron import scheduler

        self._scheduler = scheduler
        self._original_deliver = scheduler._deliver_result
        scheduler._deliver_result = self._deliver_result
        self._task = asyncio.create_task(self._tick_loop())

    async def stop(self) -> None:
        if self._task is not None:
            self._task.cancel()
            await asyncio.gather(self._task, return_exceptions=True)
        if self._scheduler is not None and self._original_deliver is not None:
            self._scheduler._deliver_result = self._original_deliver

    async def _tick_loop(self) -> None:
        while True:
            await asyncio.sleep(self.config.cron_tick_seconds)
            try:
                await asyncio.to_thread(self._scheduler.tick, False)
            except asyncio.CancelledError:
                raise
            except Exception:
                logger.exception("native Hermes cron tick failed")

    def _deliver_result(self, job: dict[str, Any], content: str, adapters: Any = None, loop: Any = None) -> str | None:
        origin = job.get("origin") or {}
        deliver = str(job.get("deliver") or "local")
        if isinstance(origin, dict) and origin.get("platform") == "app" and "origin" in deliver.split(","):
            target = str(origin.get("chat_id") or "").strip()
            agent_id = str(origin.get("chat_name") or self.config.app_agent_id).strip()
            if not target or not agent_id:
                return "app cron delivery target is incomplete"
            future = asyncio.run_coroutine_threadsafe(
                self.client.send(
                    "notify",
                    to=agent_id,
                    payload={"channel": "app", "to": target, "content": content},
                ),
                self.loop,
            )
            try:
                future.result(timeout=30)
                return None
            except Exception as exc:
                return f"app cron delivery failed: {exc}"
        return self._original_deliver(job, content, adapters=adapters, loop=loop)
