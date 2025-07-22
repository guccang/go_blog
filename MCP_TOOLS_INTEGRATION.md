# MCP工具集成到智能助手

本文档描述了如何将MCP (Model Context Protocol) 工具集成到智能助手中，使大模型能够调用外部工具和服务。

## 功能概述

智能助手现在支持MCP工具调用，大模型可以：
- 自动检测用户请求中的工具需求
- 调用已配置的MCP工具
- 处理工具返回的结果
- 基于工具结果提供智能响应

## 实现架构

### 1. 后端实现

#### MCP工具管理 (`pkgs/mcp/tools.go`)
```go
// 核心功能
- GetAvailableTools() - 获取可用工具列表
- CallTool() - 执行工具调用
- FormatToolsForLLM() - 格式化工具信息给LLM
- TestMCPServer() - 测试MCP服务器连接
```

### 2. 前端实现

#### 智能助手界面增强
- 添加MCP工具状态显示
- 工具快捷按钮
- 工具调用可视化反馈
- 工具选择对话框

#### JavaScript功能扩展
```javascript
// 新增功能
- loadMCPTools() - 加载MCP工具
- updateMCPToolsStatus() - 更新工具状态显示
- showMCPToolsDialog() - 显示工具选择对话框
```

## 工具调用流程

### 1. 用户请求处理
```
用户输入 → 智能助手 → 大模型分析 → 工具调用检测
```

### 2. 工具调用执行
```
工具检测 → MCP服务器调用 → 结果返回 → 上下文更新
```

### 3. 智能响应生成
```
工具结果 → 再次调用大模型 → 生成基于结果的响应 → 用户反馈
```

## 工具调用格式

### 大模型工具调用响应格式
```json
{
  "tool_call": {
    "name": "server.tool_name",
    "arguments": {
      "parameter1": "value1",
      "parameter2": "value2"
    }
  }
}
```

### MCP服务器通信格式
```json
// 请求
{
  "jsonrpc": "2.0",
  "id": "req_123456",
  "method": "tools/call",
  "params": {
    "name": "tool_name",
    "arguments": {
      "parameter1": "value1"
    }
  }
}

// 响应
{
  "jsonrpc": "2.0",
  "id": "req_123456",
  "result": {
    "content": "tool execution result"
  }
}
```

## API接口

### 1. MCP工具管理
- `GET /api/mcp/tools` - 获取可用工具列表
- `POST /api/mcp/tools` - 测试工具调用

### 2. 智能助手对话
- `POST /api/assistant/chat` - 支持工具调用的对话接口

## 用户界面功能

### 1. 工具状态显示
- 智能面板显示当前可用工具数量
- 工具列表和描述
- 工具状态实时更新

### 2. 工具快捷操作
- 快速访问按钮
- 工具选择对话框
- 工具参数提示

### 3. 对话增强
- 工具调用可视化提示
- 工具执行状态反馈
- 基于工具结果的智能响应

## 使用示例

### 1. 文件系统操作
```
用户: "请帮我读取项目目录下的README.md文件"
助手: 🔧 正在调用工具: filesystem.read_file
✅ 工具执行成功：已读取README.md文件内容
助手: 我已经读取了您的README.md文件...
```

### 2. 数据库查询
```
用户: "查询最近一周的用户注册数据"
助手: 🔧 正在调用工具: database.query
✅ 工具执行成功：查询到105条记录
助手: 根据查询结果，最近一周有105个新用户注册...
```

### 3. API调用
```
用户: "获取当前天气信息"
助手: 🔧 正在调用工具: weather.get_current
✅ 工具执行成功：获取到当前天气数据
助手: 当前天气：晴朗，温度22°C，湿度65%...
```

## 配置要求

### 1. MCP服务器配置
```json
{
  "name": "filesystem",
  "command": "node",
  "args": ["/path/to/mcp-server-filesystem/dist/index.js", "/allowed/directory"],
  "environment": {
    "NODE_ENV": "production"
  },
  "enabled": true,
  "description": "文件系统访问服务器"
}
```

### 2. 大模型API配置
```
deepseek_api_url=https://api.deepseek.com/v1/chat/completions
deepseek_api_key=your_api_key_here
```

## 安全考虑

### 1. 权限控制
- 用户身份验证
- 工具访问权限限制
- 敏感操作确认

### 2. 参数验证
- 工具参数类型检查
- 输入数据清理
- 执行结果过滤

### 3. 错误处理
- 工具调用超时保护
- 错误信息安全化
- 异常情况恢复

## 扩展功能

### 1. 工具链支持
- 多工具协作
- 工具调用依赖管理
- 复杂任务分解

### 2. 工具学习
- 工具使用统计
- 用户偏好记录
- 智能工具推荐

### 3. 工具生态
- 工具市场
- 社区工具分享
- 工具评级系统

## 故障排除

### 1. 工具调用失败
- 检查MCP服务器状态
- 验证工具配置
- 查看错误日志

### 2. 响应解析错误
- 检查大模型响应格式
- 验证JSON解析
- 调试工具调用逻辑

### 3. 性能问题
- 工具调用超时设置
- 并发调用限制
- 缓存机制优化

## 技术栈

- **后端**: Go 1.21, MCP协议, DeepSeek API
- **前端**: JavaScript ES6+, 实时UI更新
- **通信**: HTTP/HTTPS, JSON-RPC 2.0
- **安全**: 认证授权, 参数验证, 错误处理

MCP工具集成为智能助手提供了强大的扩展能力，使其能够与外部系统和服务进行交互，为用户提供更智能、更实用的服务。