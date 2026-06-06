#!/usr/bin/env python3
"""Run Hermes as a go_blog UAP agent."""

from __future__ import annotations

import argparse
import asyncio
import logging
import os
from pathlib import Path

from config import Config
from runtime import HermesRuntime
from uap_client import UAPClient


async def run(config: Config) -> None:
    config.validate()
    os.chdir(Path(config.workspace_dir).expanduser().resolve())
    runtime = HermesRuntime(config)
    client = UAPClient(
        config.gateway_url,
        config.agent_id,
        config.agent_name,
        config.auth_token,
        config.max_concurrent,
        os.getcwd(),
        runtime.handle_message,
    )
    runtime.bind(client)
    await runtime.start()
    try:
        await client.run()
    finally:
        await client.stop()
        await runtime.stop()


def main() -> None:
    parser = argparse.ArgumentParser(description="Hermes UAP Agent")
    parser.add_argument("--config", default="hermes-agent.json")
    parser.add_argument("--genconf", action="store_true")
    args = parser.parse_args()

    if args.genconf:
        Config().write(args.config)
        return

    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
    )
    config = Config.load(args.config)
    try:
        asyncio.run(run(config))
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()
