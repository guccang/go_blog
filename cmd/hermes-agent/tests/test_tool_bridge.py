from __future__ import annotations

import asyncio
import json
import tempfile
import unittest
from pathlib import Path

from config import Config
from tool_bridge import gateway_http_url, sanitize_tool_name
from uap_client import UAPClient


class HelperTests(unittest.TestCase):
    def test_gateway_http_url(self) -> None:
        self.assertEqual(
            gateway_http_url("ws://127.0.0.1:9000/ws/uap"), "http://127.0.0.1:9000"
        )
        self.assertEqual(
            gateway_http_url("wss://blog.example.com:10086/ws/uap"),
            "https://blog.example.com:10086",
        )

    def test_sanitize_tool_name(self) -> None:
        self.assertEqual(sanitize_tool_name("app.SendMessage"), "app_SendMessage")
        self.assertEqual(sanitize_tool_name("GetBlogsByTag"), "GetBlogsByTag")


class ToolResultRoutingTests(unittest.TestCase):
    def _client(self) -> UAPClient:
        async def handler(_message):
            return None

        return UAPClient("ws://x/ws/uap", "hermes-agent", "Hermes", "", 1, ".", handler)

    def test_handle_tool_result_completes_pending_future(self) -> None:
        async def scenario() -> dict:
            client = self._client()
            future: asyncio.Future = asyncio.get_running_loop().create_future()
            client._pending_tool_calls["msg-1"] = future
            matched = client.handle_tool_result(
                {
                    "type": "tool_result",
                    "id": "msg-1",
                    "payload": {"request_id": "msg-1", "success": True, "result": "ok"},
                }
            )
            self.assertTrue(matched)
            return await future

        payload = asyncio.run(scenario())
        self.assertTrue(payload["success"])
        self.assertEqual(payload["result"], "ok")

    def test_handle_tool_result_unmatched(self) -> None:
        client = self._client()
        matched = client.handle_tool_result(
            {"type": "tool_result", "payload": {"request_id": "unknown"}}
        )
        self.assertFalse(matched)


class PluginInstallTests(unittest.TestCase):
    def test_prepare_native_home_installs_go_blog_plugin(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            config = Config(hermes_home=str(Path(tmp) / "hermes"), workspace_dir=tmp)
            config.prepare_native_home()
            plugin_dir = Path(config.hermes_home) / "plugins" / "go_blog"
            self.assertTrue((plugin_dir / "plugin.yaml").exists())
            self.assertTrue((plugin_dir / "__init__.py").exists())
            native = json.loads((Path(config.hermes_home) / "config.yaml").read_text("utf-8"))
            self.assertIn("go_blog", native["plugins"]["enabled"])

    def test_prepare_native_home_skips_plugin_when_disabled(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            config = Config(
                hermes_home=str(Path(tmp) / "hermes"),
                workspace_dir=tmp,
                tool_bridge_enabled=False,
            )
            config.prepare_native_home()
            plugin_dir = Path(config.hermes_home) / "plugins" / "go_blog"
            self.assertFalse(plugin_dir.exists())
            native = json.loads((Path(config.hermes_home) / "config.yaml").read_text("utf-8"))
            self.assertNotIn("plugins", native)


if __name__ == "__main__":
    unittest.main()
