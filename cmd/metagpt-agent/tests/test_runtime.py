from __future__ import annotations

import unittest

from metagpt_agent.runtime import is_greeting, sanitize_tool_name, unsanitize_tool_name


class RuntimeHelpersTest(unittest.TestCase):
    def test_tool_name_roundtrip(self) -> None:
        original = "wechat.SendMessage"
        sanitized = sanitize_tool_name(original)
        self.assertEqual(sanitized, "wechat_SendMessage")
        self.assertEqual(unsanitize_tool_name(sanitized), original)

    def test_greeting_detection(self) -> None:
        self.assertTrue(is_greeting("你好"))
        self.assertTrue(is_greeting("hello"))
        self.assertFalse(is_greeting("帮我部署 blog-agent"))


if __name__ == "__main__":
    unittest.main()
