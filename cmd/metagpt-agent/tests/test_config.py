from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from metagpt_agent.config import AgentConfig


class ConfigTest(unittest.TestCase):
    def test_load_defaults_when_file_missing(self) -> None:
        config = AgentConfig.load("not-found.json")
        self.assertEqual(config.agent_id, "metagpt-agent")

    def test_load_custom_llm_values(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            path = Path(temp_dir) / "config.json"
            path.write_text(
                '{"agent_id":"mg","llm":{"model":"deepseek-chat","api_key":"k"}}',
                encoding="utf-8",
            )
            config = AgentConfig.load(str(path))
            self.assertEqual(config.agent_id, "mg")
            self.assertEqual(config.llm.model, "deepseek-chat")
            self.assertEqual(config.llm.api_key, "k")


if __name__ == "__main__":
    unittest.main()
