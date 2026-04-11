from __future__ import annotations

import tempfile
import unittest

from metagpt_agent.session_store import SessionStore


class SessionStoreTest(unittest.TestCase):
    def test_save_and_load_task_files(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            store = SessionStore(f"{temp_dir}/sessions", f"{temp_dir}/chat")
            payload = {"status": "running", "messages": [{"role": "user", "content": "hi"}]}
            store.save_session("root-1", "root-1", payload)
            loaded = store.load_session("root-1", "root-1")
            self.assertEqual(payload, loaded)

    def test_save_and_load_chat_session(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            store = SessionStore(f"{temp_dir}/sessions", f"{temp_dir}/chat")
            payload = {"source": "wechat", "user": "alice", "messages": [{"role": "assistant", "content": "ok"}]}
            store.save_chat_session("wechat", "alice", payload)
            loaded = store.load_chat_session("wechat", "alice")
            self.assertEqual(payload, loaded)


if __name__ == "__main__":
    unittest.main()
