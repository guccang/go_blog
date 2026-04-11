from __future__ import annotations

import argparse
import asyncio
import logging
import sys

from metagpt_agent.config import AgentConfig
from metagpt_agent.service import MetaGPTAgentService


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="MetaGPT-based UAP agent")
    parser.add_argument("--config", default="metagpt-agent.json", help="Path to config JSON")
    return parser.parse_args()


async def async_main() -> int:
    args = parse_args()
    config = AgentConfig.load(args.config)
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s %(message)s",
    )
    service = MetaGPTAgentService(config)
    await service.start()
    return 0


def main() -> int:
    try:
        return asyncio.run(async_main())
    except KeyboardInterrupt:
        return 130


if __name__ == "__main__":
    sys.exit(main())
