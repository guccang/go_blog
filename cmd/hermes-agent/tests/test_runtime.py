from __future__ import annotations

import asyncio
import json
import os
import tempfile
import unittest
from dataclasses import fields
from pathlib import Path
from typing import Any

from config import Config
from cron_bridge import CronBridge
from native_runtime import runtime_source
from runtime import (
    HermesRuntime,
    extract_final_response,
    extract_task_account,
    extract_task_query,
    normalize_app_content,
    normalize_progress_text,
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
    def test_config_excludes_internal_fields_when_loading(self) -> None:
        init_fields = {item.name for item in fields(Config) if item.init}
        self.assertNotIn("_config_dir", init_fields)

    def test_config_resolves_embedded_and_state_paths(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config_path = root / "config" / "hermes-agent.json"
            config_path.parent.mkdir()
            config_path.write_text(
                json.dumps(
                    {
                        "workspace_dir": "..",
                        "embedded_source": "vendor/hermes_runtime",
                        "hermes_home": "state/hermes",
                        "session_dir": "sessions",
                    }
                ),
                encoding="utf-8",
            )
            config = Config.load(config_path)
            config.resolve_paths()
            self.assertEqual(root.resolve(), Path(config.workspace_dir))
            self.assertEqual(
                (config_path.parent / "vendor/hermes_runtime").resolve(),
                Path(config.embedded_source),
            )
            self.assertEqual((root / "state/hermes").resolve(), Path(config.hermes_home))
            self.assertEqual((root / "sessions").resolve(), Path(config.session_dir))

    def test_embedded_runtime_does_not_require_external_source(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            embedded = Path(directory) / "embedded"
            embedded.mkdir()
            (embedded / "run_agent.py").write_text("class AIAgent: pass\n", encoding="utf-8")
            config = Config(
                runtime_mode="embedded",
                embedded_source=str(embedded),
                hermes_source=str(Path(directory) / "missing-external"),
            )
            self.assertEqual(embedded.resolve(), runtime_source(config))

    def test_prepare_native_home_writes_model_env_for_cron(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            home = Path(directory) / "hermes"
            config = Config(
                hermes_home=str(home),
                workspace_dir=directory,
                model="deepseek-v4-flash",
                provider="deepseek",
                base_url="https://api.deepseek.com/v1",
                api_key="sk-test",
                native_config={"model": {"provider": "auto"}},
            )
            previous_key = os.environ.get("DEEPSEEK_API_KEY")
            previous_base = os.environ.get("DEEPSEEK_BASE_URL")
            try:
                config.prepare_native_home()
                native = json.loads((home / "config.yaml").read_text(encoding="utf-8"))
                self.assertEqual("deepseek", native["model"]["provider"])
                self.assertEqual("deepseek-v4-flash", native["model"]["default"])
                self.assertEqual("https://api.deepseek.com/v1", native["model"]["base_url"])
                self.assertEqual("sk-test", native["model"]["api_key"])
                env_text = (home / ".env").read_text(encoding="utf-8")
                self.assertIn("DEEPSEEK_API_KEY=sk-test", env_text)
                self.assertIn("DEEPSEEK_BASE_URL=https://api.deepseek.com/v1", env_text)
                self.assertEqual("sk-test", os.environ.get("DEEPSEEK_API_KEY"))
            finally:
                if previous_key is None:
                    os.environ.pop("DEEPSEEK_API_KEY", None)
                else:
                    os.environ["DEEPSEEK_API_KEY"] = previous_key
                if previous_base is None:
                    os.environ.pop("DEEPSEEK_BASE_URL", None)
                else:
                    os.environ["DEEPSEEK_BASE_URL"] = previous_base

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

    def test_lifecycle_progress_is_hidden(self) -> None:
        self.assertEqual("", normalize_progress_text("lifecycle"))
        self.assertEqual("", normalize_progress_text(" LIFECYCLE "))
        self.assertEqual("正在调用工具", normalize_progress_text("正在调用工具"))

    def test_extract_final_response_falls_back_to_last_assistant_message(self) -> None:
        result = {
            "final_response": "",
            "messages": [
                {"role": "user", "content": "你好"},
                {
                    "role": "assistant",
                    "content": [
                        {"type": "text", "text": "你好，"},
                        {"type": "text", "text": "有什么可以帮你？"},
                    ],
                },
            ],
        }
        self.assertEqual("你好，\n有什么可以帮你？", extract_final_response(result))


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


class CronBridgeTests(unittest.IsolatedAsyncioTestCase):
    async def test_app_origin_result_returns_to_original_user(self) -> None:
        client = FakeClient()
        bridge = CronBridge(Config(app_agent_id="app-app-agent"), client, asyncio.get_running_loop())
        bridge._original_deliver = lambda *_args, **_kwargs: "native"

        result = await asyncio.to_thread(
            bridge._deliver_result,
            {
                "deliver": "origin",
                "origin": {
                    "platform": "app",
                    "chat_id": "alice",
                    "chat_name": "custom-app-agent",
                },
            },
            "cron completed",
        )

        self.assertIsNone(result)
        self.assertEqual(
            [
                (
                    "notify",
                    "custom-app-agent",
                    {"channel": "app", "to": "alice", "content": "cron completed"},
                )
            ],
            client.messages,
        )


if __name__ == "__main__":
    unittest.main()
