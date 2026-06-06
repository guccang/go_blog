from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path
from typing import Any

from config import Config
from runtime import (
    HermesRuntime,
    extract_task_account,
    extract_task_query,
    normalize_app_content,
)


class FakeAgent:
    calls: list[dict[str, Any]] = []

    def __init__(self, **kwargs: Any) -> None:
        self.kwargs = kwargs
        self.interrupted = False

    def interrupt(self, _: str) -> None:
        self.interrupted = True

    def run_conversation(self, query: str, **kwargs: Any) -> dict[str, Any]:
        self.calls.append({"query": query, **kwargs})
        history = list(kwargs.get("conversation_history") or [])
        messages = history + [
            {"role": "user", "content": query},
            {"role": "assistant", "content": f"done:{query}"},
        ]
        return {"final_response": f"done:{query}", "messages": messages}


class FakeClient:
    def __init__(self) -> None:
        self.messages: list[tuple[str, str, dict[str, Any]]] = []

    async def send(self, message_type: str, *, to: str, payload: dict[str, Any]) -> None:
        self.messages.append((message_type, to, payload))


class ParsingTests(unittest.TestCase):
    def test_normalize_app_json(self) -> None:
        content = "APP_MESSAGE_JSON:" + json.dumps(
            {
                "content": "处理这个任务",
                "group_id": "g1",
                "attachment": {"file_name": "spec.md", "file_path": "/tmp/spec.md"},
            },
            ensure_ascii=False,
        )
        normalized = normalize_app_content(content)
        self.assertIn("处理这个任务", normalized)
        self.assertIn("/tmp/spec.md", normalized)
        self.assertIn("group_id=g1", normalized)

    def test_extract_task_query_from_messages(self) -> None:
        payload = {
            "task_id": "t1",
            "payload": {
                "task_type": "llm_request",
                "account": "alice",
                "messages": [{"role": "user", "content": "finish it"}],
            },
        }
        task_id, query = extract_task_query(payload)
        self.assertEqual("t1", task_id)
        self.assertEqual("finish it", query)
        self.assertEqual("alice", extract_task_account(payload, task_id))


class RuntimeTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self) -> None:
        FakeAgent.calls.clear()
        self.temp = tempfile.TemporaryDirectory()
        config = Config(
            session_dir=self.temp.name,
            max_concurrent=1,
            task_queue_size=2,
        )
        self.runtime = HermesRuntime(config, agent_class=FakeAgent)
        self.client = FakeClient()
        self.runtime.bind(self.client)
        await self.runtime.start()

    async def asyncTearDown(self) -> None:
        await self.runtime.stop()
        self.temp.cleanup()

    async def test_app_notify_runs_and_reuses_history(self) -> None:
        message = {
            "type": "notify",
            "from": "app-app-agent",
            "payload": {"channel": "app", "to": "alice", "content": "first"},
        }
        await self.runtime.handle_message(message)
        await self.runtime.queue.join()
        message["payload"]["content"] = "second"
        await self.runtime.handle_message(message)
        await self.runtime.queue.join()

        self.assertEqual(2, len(FakeAgent.calls))
        self.assertEqual([], FakeAgent.calls[0]["conversation_history"])
        self.assertEqual(2, len(FakeAgent.calls[1]["conversation_history"]))
        self.assertEqual("notify", self.client.messages[-1][0])
        self.assertEqual("done:second", self.client.messages[-1][2]["content"])
        self.assertTrue((Path(self.temp.name) / "alice.json").exists())

    async def test_task_assign_sends_accepted_and_complete(self) -> None:
        await self.runtime.handle_message(
            {
                "type": "task_assign",
                "from": "gateway-client",
                "payload": {
                    "task_id": "task-1",
                    "payload": {"task_type": "assistant_chat", "query": "ship it"},
                },
            }
        )
        await self.runtime.queue.join()
        types = [message[0] for message in self.client.messages]
        self.assertEqual(["task_accepted", "task_complete"], types)
        self.assertEqual("success", self.client.messages[-1][2]["status"])

    async def test_stopped_queued_task_is_cancelled(self) -> None:
        await self.runtime.handle_message(
            {"type": "task_stop", "payload": {"task_id": "task-stop"}}
        )
        await self.runtime.handle_message(
            {
                "type": "task_assign",
                "from": "gateway-client",
                "payload": {
                    "task_id": "task-stop",
                    "payload": {"task_type": "assistant_chat", "query": "do not run"},
                },
            }
        )
        await self.runtime.queue.join()
        self.assertEqual("cancelled", self.client.messages[-1][2]["status"])


if __name__ == "__main__":
    unittest.main()
