"""go_blog Hermes 插件：注册 blog-agent 工具（实现见 hermes-agent/tool_bridge.py）。"""

from __future__ import annotations

from typing import Any


def register(ctx: Any) -> None:
    import tool_bridge

    tool_bridge.register_plugin_tools(ctx)
