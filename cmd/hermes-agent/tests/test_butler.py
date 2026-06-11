from __future__ import annotations

import sys
import types
import unittest

from butler import BUTLER_JOBS, BUTLER_RULES, ensure_butler_jobs
from config import Config


class FakeCronJobs:
    def __init__(self, existing_names: list[str]) -> None:
        self.existing = [{"name": name} for name in existing_names]
        self.created: list[dict] = []

    def load_jobs(self):
        return list(self.existing)

    def create_job(self, **kwargs):
        self.created.append(kwargs)
        return {"id": "fake", **kwargs}


def install_fake_cron(fake: FakeCronJobs) -> None:
    cron_pkg = types.ModuleType("cron")
    jobs_mod = types.ModuleType("cron.jobs")
    jobs_mod.load_jobs = fake.load_jobs
    jobs_mod.create_job = fake.create_job
    cron_pkg.jobs = jobs_mod
    sys.modules["cron"] = cron_pkg
    sys.modules["cron.jobs"] = jobs_mod


class ButlerJobsTests(unittest.TestCase):
    def tearDown(self) -> None:
        sys.modules.pop("cron", None)
        sys.modules.pop("cron.jobs", None)

    def test_creates_jobs_for_accounts(self) -> None:
        fake = FakeCronJobs([])
        install_fake_cron(fake)
        config = Config(butler_accounts=["alice"])
        created = ensure_butler_jobs(config)
        self.assertEqual(created, len(BUTLER_JOBS))
        names = {job["name"] for job in fake.created}
        self.assertIn("butler-goal-review-alice", names)
        self.assertIn("butler-memory-consolidation-alice", names)
        self.assertIn("butler-habit-reminder-alice", names)
        for job in fake.created:
            self.assertEqual(job["deliver"], "origin")
            self.assertEqual(job["origin"]["platform"], "app")
            self.assertEqual(job["origin"]["chat_id"], "alice")
            self.assertIn("alice", job["prompt"])

    def test_idempotent_when_jobs_exist(self) -> None:
        fake = FakeCronJobs(
            [
                "butler-goal-review-alice",
                "butler-memory-consolidation-alice",
                "butler-habit-reminder-alice",
            ]
        )
        install_fake_cron(fake)
        config = Config(butler_accounts=["alice"])
        self.assertEqual(ensure_butler_jobs(config), 0)
        self.assertEqual(fake.created, [])

    def test_no_accounts_no_jobs(self) -> None:
        fake = FakeCronJobs([])
        install_fake_cron(fake)
        self.assertEqual(ensure_butler_jobs(Config()), 0)

    def test_effective_system_prompt_appends_rules(self) -> None:
        config = Config(system_prompt="基础提示", butler_enabled=True)
        self.assertTrue(config.effective_system_prompt().startswith("基础提示"))
        self.assertIn("私人管家职责", config.effective_system_prompt())
        disabled = Config(system_prompt="基础提示", butler_enabled=False)
        self.assertEqual(disabled.effective_system_prompt(), "基础提示")
        self.assertIn("RawMemoryAppend", BUTLER_RULES)


if __name__ == "__main__":
    unittest.main()
