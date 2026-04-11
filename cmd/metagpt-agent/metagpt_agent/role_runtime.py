from __future__ import annotations

from typing import Any

from .config import AgentConfig
from .runtime import EventSink, QueryExecutor, TaskExecutionResult, TaskRequest, ToolDescriptor, ToolInvoker

try:
    from metagpt.actions import Action
    from metagpt.config2 import Config as MetaGPTConfig
    from metagpt.roles.role import Role, RoleReactMode
    from metagpt.schema import Message as MetaGPTMessage

    METAGPT_AVAILABLE = True
except ImportError:
    Action = object  # type: ignore[assignment]
    MetaGPTConfig = None  # type: ignore[assignment]
    Role = object  # type: ignore[assignment]
    RoleReactMode = None  # type: ignore[assignment]
    MetaGPTMessage = None  # type: ignore[assignment]
    METAGPT_AVAILABLE = False


def build_metagpt_config(config: AgentConfig) -> Any:
    if not METAGPT_AVAILABLE:
        raise RuntimeError(
            "MetaGPT is not installed. Install dependencies with `pip install -r requirements.txt` first."
        )
    return MetaGPTConfig.from_llm_config(
        {
            "api_key": config.llm.api_key,
            "api_type": "openai",
            "base_url": config.llm.base_url,
            "model": config.llm.model,
            "max_token": config.llm.max_tokens,
            "temperature": config.llm.temperature,
            "stream": False,
            "timeout": config.llm.timeout_sec,
        }
    )


if METAGPT_AVAILABLE:

    class ExecuteQueryLoopAction(Action):
        name: str = "ExecuteQueryLoopAction"

        async def run(
            self,
            request: TaskRequest,
            executor: QueryExecutor,
            tools: list[ToolDescriptor],
            sink: EventSink,
            tool_invoker: ToolInvoker,
        ) -> TaskExecutionResult:
            return await executor.execute(request, tools, sink, tool_invoker)


    class MetaGPTTaskRole(Role):
        def __init__(self, config: AgentConfig, executor: QueryExecutor, **kwargs: Any) -> None:
            meta_config = build_metagpt_config(config)
            super().__init__(
                name="MetaGPTTaskRole",
                profile="MetaGPT UAP Task Executor",
                goal="Finish assigned UAP tasks with real tool execution and concise summaries",
                constraints="Only rely on actual tool outputs and never fabricate execution results",
                config=meta_config,
                **kwargs,
            )
            self.executor = executor
            self.current_request: TaskRequest | None = None
            self.current_tools: list[ToolDescriptor] = []
            self.current_sink: EventSink | None = None
            self.current_tool_invoker: ToolInvoker | None = None
            self.last_result: TaskExecutionResult | None = None
            self.set_actions([ExecuteQueryLoopAction])
            self._set_react_mode(RoleReactMode.BY_ORDER.value)

        async def _act(self) -> MetaGPTMessage:
            todo = self.rc.todo
            if not self.current_request or not self.current_sink or not self.current_tool_invoker:
                raise RuntimeError("MetaGPTTaskRole is missing execution context")
            result = await todo.run(
                self.current_request,
                self.executor,
                self.current_tools,
                self.current_sink,
                self.current_tool_invoker,
            )
            self.last_result = result
            message = MetaGPTMessage(content=result.text, role="assistant", cause_by=type(todo))
            self.rc.memory.add(message)
            return message

        async def run_request(
            self,
            request: TaskRequest,
            tools: list[ToolDescriptor],
            sink: EventSink,
            tool_invoker: ToolInvoker,
        ) -> TaskExecutionResult:
            self.current_request = request
            self.current_tools = tools
            self.current_sink = sink
            self.current_tool_invoker = tool_invoker
            self.last_result = None
            prompt = request.query or self.executor.infer_query(request.messages) or request.task_type
            await self.run(prompt)
            if self.last_result is None:
                raise RuntimeError("MetaGPT role completed without task result")
            return self.last_result

else:

    class MetaGPTTaskRole:  # type: ignore[no-redef]
        def __init__(self, *_args: Any, **_kwargs: Any) -> None:
            raise RuntimeError(
                "MetaGPT is not installed. Install dependencies with `pip install -r requirements.txt` first."
            )
