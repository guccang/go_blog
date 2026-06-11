"""私人管家（Butler）支撑：记忆规范提示词 + 周期任务的幂等创建。

设计参考 MiMo Code 长程任务实践：
- 记忆：Markdown 可审阅文件（MEMORY/checkpoint/journal），写入规范交给系统提示词
- 进化：独立判官周期评审目标判据（防"乐观停止"）+ 周记忆整理（防膨胀）
"""

from __future__ import annotations

import logging
from typing import Any

logger = logging.getLogger(__name__)

# 追加到 Hermes 系统提示词的管家行为规范
BUTLER_RULES = """

【私人管家职责】你同时是用户的私人管家，遵守以下规范：
1. 记忆：发现值得长期记住的事实（用户偏好、习惯、忌讳、重要的人、纪念日、健康状况、承诺）时，
   调用 Inner_blog_RawMemoryAppend(file="MEMORY") 记录，一条一个简洁事实；
   当天发生的事、用户情绪观察、完成的事项，用 Inner_blog_RawMemoryJournal 写当日日志；
   回答涉及用户个人背景时，先用 Inner_blog_RawMemorySearch 检索记忆再回答；
   不记录超出需要的隐私内容。
2. 目标：用户提出目标时，先确认可客观验证的完成判据（追问到能写成"判据: ..."为止），
   再用 Inner_blog_RawSaveGoal / Inner_blog_RawAddGoalTask 写入（判据放 overview/description），
   并用 Inner_blog_RawMemoryWrite 更新 checkpoint 中的当前关注点。
3. 提醒：用户要求提醒时，用 cronjob 工具创建定时任务（一次性提醒 repeat=1），
   提醒文案应具体、友好；提醒创建成功后向用户复述时间和内容。
4. 风格：像可靠的管家——简洁、温和、主动汇报结果；用户情绪低落时先共情再办事。
"""

# 周日 21:40 目标评审（独立判官）
GOAL_REVIEW_JOB = (
    "butler-goal-review-{account}",
    "40 21 * * 0",
    """你是严格的目标评审判官，为账号 {account} 做每周目标评审：
1. 调用 Inner_blog_RawGetCurrentGoals、Inner_blog_RawGetWeeklyGoal、Inner_blog_RawGetMonthlyGoal 获取目标与任务（account="{account}"）。
2. 对每个目标，依据 overview/description 中"判据:"后的客观标准逐条核对；需要事实数据时调用
   Inner_blog_RawGetTodosByDate、Inner_blog_RawGetExerciseStats 等工具核实。
   只回答判据是否客观达成，禁止"差不多就算完成"。
3. 用 Inner_blog_RawMemoryJournal 记录评审结论；读取 checkpoint（Inner_blog_RawMemoryRead file="checkpoint"）
   后用 Inner_blog_RawMemoryWrite 更新下周关注点。
4. 最后输出给用户的简短播报（≤150字）：达成的目标给予具体肯定；落后的目标给出一条可执行建议，鼓励而非指责。""",
)

# 周日 22:10 记忆整理（防膨胀）
MEMORY_CONSOLIDATION_JOB = (
    "butler-memory-consolidation-{account}",
    "10 22 * * 0",
    """为账号 {account} 做每周记忆整理：
1. Inner_blog_RawMemoryListFiles(account="{account}") 列出记忆文件，读取最近 7 天的 journal_* 文件。
2. 提炼值得长期保留的事实（偏好、习惯、重要事件、情绪模式），读取 MEMORY 文件后用
   Inner_blog_RawMemoryWrite 整理重写：合并重复条目、删除过期信息、保持分节清晰。
3. 同步更新 checkpoint 文件。
4. 输出 ≤100 字的"本周回顾"给用户：本周完成了什么、情绪如何、下周建议关注什么。""",
)

BUTLER_JOBS = (GOAL_REVIEW_JOB, MEMORY_CONSOLIDATION_JOB)


def ensure_butler_jobs(config: Any) -> int:
    """为 butler_accounts 中的账号幂等创建管家周期任务，返回新建数量。"""
    accounts = [a.strip() for a in (config.butler_accounts or []) if a.strip()]
    if not accounts:
        return 0
    try:
        from cron import jobs as cron_jobs
    except Exception:
        logger.exception("butler jobs: cron module unavailable")
        return 0

    try:
        existing = {str(job.get("name") or "") for job in cron_jobs.load_jobs()}
    except Exception:
        logger.exception("butler jobs: load_jobs failed")
        return 0

    created = 0
    for account in accounts:
        origin = {
            "platform": "app",
            "chat_id": account,
            "chat_name": config.app_agent_id,
        }
        for name_tpl, schedule, prompt_tpl in BUTLER_JOBS:
            name = name_tpl.format(account=account)
            if name in existing:
                continue
            try:
                cron_jobs.create_job(
                    prompt=prompt_tpl.format(account=account),
                    schedule=schedule,
                    name=name,
                    deliver="origin",
                    origin=origin,
                )
                created += 1
                logger.info("butler job created: %s", name)
            except Exception:
                logger.exception("butler jobs: create %s failed", name)
    return created
