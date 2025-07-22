import json
import logging

logger = logging.getLogger(__name__)

class MCPProcessor:
    def __init__(self, llm_client, tools, model_name="gpt-4", temperature=0.7):
        self.llm_client = llm_client
        self.tools = tools
        self.model_name = model_name
        self.temperature = temperature

    async def process_query(self, query: str, selected_tools: list[str]) -> str:
        """ use llm and mcp server tools process query """
        messages = [
            {
                "role": "system",
                "content": "你是一个万能助手，自行决定是否调用工具获取数据，当你得到工具返回结果后，就不需要调用相同工具了，最后返回简单直接的结果给用户。",
            },
            { 
                "role": "user",
                "content": f"{query}",
            }
        ]
        logger.info(f"### llm response:\n {messages} {selected_tools}")

        available_tools = [{
            "type": "function",
            "function": {
                "name": tool.name,
                "description": tool.description,
                "parameters": tool.inputSchema
            }
        } for tool in self.tools if selected_tools is None or tool.name in selected_tools]

        logger.info(f"### send to llm:\n {messages}")
        # 初始化 LLM API 调用
        response = await self.llm_client.chat.completions.create(
            model=self.model_name,
            messages=messages,
            tools=available_tools,
            temperature=self.temperature,
        )

        final_text = []
        message = response.choices[0].message
        final_text.append(message.content or "")
        logger.info(f"### llm response:\n {message}")

        # call tools
        max_call = 25
        while message.tool_calls and max_call > 0:
            max_call = max_call - 1
            # call tool
            for tool_call in message.tool_calls:
                tool_name = tool_call.function.name
                tool_args = json.loads(tool_call.function.arguments)
                result = await self.call_tool(tool_name, tool_args)
                final_text.append(f"[Calling tool {tool_name} with args {tool_args}]\n")
                logger.info(f"### call tool\n {tool_name} {tool_args} {result}")
                # 将工具调用和结果添加到消息历史
                messages.append({
                    "role": "assistant",
                    "tool_calls": [{
                        "id": tool_call.id,
                        "type": "function",
                        "function": {
                            "name": tool_name,
                            "arguments": json.dumps(tool_args)
                        }
                    }]
                })

                messages.append({
                    "role": "tool",
                    "tool_call_id": tool_call.id,
                    "content": str(result.content)
                })

            logger.info(f"### send to llm:\n {messages}")
            response = await self.llm_client.chat.completions.create(
                model=self.model_name,
                messages=messages,
                tools=available_tools,
                temperature=self.temperature,
            )

            message = response.choices[0].message
            logger.info(f"### llm response:\n {message}")
            if message.content:
                final_text.append(message.content)

        return "\n".join(final_text)

    async def call_tool(self, tool_name: str, tool_args: dict):
        """Call a specific tool by name with given arguments"""
        for tool in self.tools:
            if tool.name == tool_name:
                return await tool.call(tool_args)
        raise ValueError(f"Tool {tool_name} not found")