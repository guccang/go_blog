from __future__ import annotations

import json
import time
import uuid
from dataclasses import asdict, dataclass, field
from typing import Any, Dict, Optional


def new_msg_id() -> str:
    return uuid.uuid4().hex


def json_dumps(data: Any) -> str:
    return json.dumps(data, ensure_ascii=False, separators=(",", ":"))


@dataclass
class MessageEnvelope:
    type: str
    id: str
    from_id: str
    to: str = ""
    payload: Any = field(default_factory=dict)
    ts: int = field(default_factory=lambda: int(time.time() * 1000))

    def to_wire(self) -> str:
        return json_dumps(
            {
                "type": self.type,
                "id": self.id,
                "from": self.from_id,
                "to": self.to,
                "payload": self.payload,
                "ts": self.ts,
            }
        )

    @classmethod
    def from_wire(cls, raw: str) -> "MessageEnvelope":
        data = json.loads(raw)
        return cls(
            type=data.get("type", ""),
            id=data.get("id", ""),
            from_id=data.get("from", ""),
            to=data.get("to", ""),
            payload=data.get("payload", {}),
            ts=data.get("ts", 0),
        )


@dataclass
class ToolDef:
    name: str
    description: str
    parameters: Dict[str, Any]

    def to_dict(self) -> Dict[str, Any]:
        return {
            "name": self.name,
            "description": self.description,
            "parameters": self.parameters,
        }


@dataclass
class RegisterPayload:
    agent_id: str
    agent_type: str
    name: str
    description: str
    host_platform: str
    host_ip: str
    workspace: str
    tools: list[Dict[str, Any]]
    capacity: int
    meta: Dict[str, Any]
    auth_token: str = ""

    def to_dict(self) -> Dict[str, Any]:
        return asdict(self)


@dataclass
class TaskAssignPayload:
    task_id: str
    payload: Dict[str, Any]


@dataclass
class TaskAcceptedPayload:
    task_id: str


@dataclass
class TaskRejectedPayload:
    task_id: str
    reason: str


@dataclass
class TaskCompletePayload:
    task_id: str
    status: str
    error: str = ""
    result: str = ""


@dataclass
class TaskEventPayload:
    task_id: str
    event: Dict[str, Any]


@dataclass
class NotifyPayload:
    channel: str
    to: str
    content: str
    message_type: str = ""
    meta: Optional[Dict[str, Any]] = None

    def to_dict(self) -> Dict[str, Any]:
        data = {
            "channel": self.channel,
            "to": self.to,
            "content": self.content,
        }
        if self.message_type:
            data["message_type"] = self.message_type
        if self.meta:
            data["meta"] = self.meta
        return data


@dataclass
class ToolCallPayload:
    tool_name: str
    arguments: Any
    authenticated_user: str = ""

    def to_dict(self) -> Dict[str, Any]:
        data = {
            "tool_name": self.tool_name,
            "arguments": self.arguments,
        }
        if self.authenticated_user:
            data["authenticated_user"] = self.authenticated_user
        return data


@dataclass
class ToolResultPayload:
    request_id: str
    success: bool
    result: str = ""
    error: str = ""

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "ToolResultPayload":
        return cls(
            request_id=data.get("request_id", ""),
            success=bool(data.get("success", False)),
            result=data.get("result", "") or "",
            error=data.get("error", "") or "",
        )
