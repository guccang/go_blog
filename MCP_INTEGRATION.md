# MCP (Model Context Protocol) 集成

本文档描述了如何在 Go Blog 系统中集成 MCP (Model Context Protocol) 功能。

## 功能概述

MCP 集成为智能助手系统提供了配置和管理外部工具和数据源的能力。用户可以通过 Web 界面配置各种 MCP 服务器，包括文件系统访问、数据库连接、API 集成等。

## 已实现的功能

### 后端功能

1. **MCP 配置管理**
   - 配置的增删改查 (CRUD)
   - 配置启用/禁用切换
   - 配置验证
   - JSON 配置文件存储

2. **API 接口**
   - `GET /api/mcp?action=list` - 获取所有配置
   - `GET /api/mcp?action=get&name={name}` - 获取单个配置
   - `POST /api/mcp` - 添加新配置
   - `PUT /api/mcp?name={name}` - 更新配置
   - `PUT /api/mcp?action=toggle&name={name}` - 切换配置状态
   - `DELETE /api/mcp?name={name}` - 删除配置

### 前端功能

1. **MCP 配置页面** (`/mcp`)
   - 配置列表展示
   - 配置状态指示
   - 统计信息显示

2. **配置管理界面**
   - 添加/编辑配置的模态框
   - 表单验证
   - 实时状态更新

3. **用户体验**
   - 响应式设计
   - 操作提示和通知
   - 键盘快捷键支持

## 文件结构

```
pkgs/mcp/
├── mcp.go          # 核心配置管理逻辑
├── handlers.go     # HTTP 处理器
├── go.mod          # Go 模块配置
└── build.sh        # 构建脚本

templates/
└── mcp.template    # MCP 页面模板

statics/css/
└── mcp.css         # MCP 页面样式

statics/js/
└── mcp.js          # MCP 页面交互逻辑
```

## 配置格式

MCP 配置以 JSON 格式存储在 `blogs_txt/mcp_config.json` 文件中：

```json
{
  "configs": [
    {
      "name": "filesystem",
      "command": "node",
      "args": ["/path/to/mcp-server/dist/index.js", "/allowed/directory"],
      "environment": {
        "NODE_ENV": "production"
      },
      "enabled": true,
      "description": "文件系统访问服务器",
      "created_at": "2025-01-01T00:00:00Z",
      "updated_at": "2025-01-01T00:00:00Z"
    }
  ]
}
```

## 配置字段说明

- `name`: 配置名称（唯一标识符）
- `command`: 要执行的命令
- `args`: 命令参数数组
- `environment`: 环境变量映射
- `enabled`: 是否启用此配置
- `description`: 配置描述
- `created_at`: 创建时间
- `updated_at`: 更新时间

## 安全考虑

1. **认证要求**: 所有 MCP 相关功能都需要用户登录认证
2. **输入验证**: 对所有用户输入进行验证和清理
3. **XSS 防护**: 前端使用 HTML 转义防止 XSS 攻击
4. **配置隔离**: 每个用户的配置独立存储

## 使用方法

1. **访问 MCP 页面**
   - 登录系统后访问 `/mcp` 页面

2. **添加新配置**
   - 点击"添加配置"按钮
   - 填写配置信息
   - 保存配置

3. **管理现有配置**
   - 启用/禁用配置
   - 编辑配置详情
   - 删除不需要的配置

## 集成到主系统

MCP 模块已完全集成到 Go Blog 系统中：

1. **主程序集成** (`main.go`)
   - 导入 MCP 模块
   - 初始化 MCP 功能
   - 输出版本信息

2. **HTTP 路由集成** (`pkgs/http/http.go`)
   - 注册 MCP 页面路由
   - 注册 MCP API 路由

3. **模块依赖** (`go.mod`)
   - 添加 MCP 模块依赖
   - 配置模块替换路径

## 扩展功能

未来可以考虑以下扩展：

1. **配置模板**: 预定义常用 MCP 服务器配置模板
2. **配置导入/导出**: 支持配置的批量导入和导出
3. **连接测试**: 提供配置连接测试功能
4. **日志记录**: 记录 MCP 操作和错误日志
5. **权限控制**: 更细粒度的权限控制机制

## 故障排除

1. **配置不生效**: 检查配置是否已启用
2. **页面无法访问**: 确认用户已登录
3. **配置保存失败**: 检查文件系统权限
4. **前端功能异常**: 检查浏览器控制台错误

## 技术栈

- **后端**: Go 1.21
- **前端**: HTML5, CSS3, JavaScript (ES6+)
- **存储**: JSON 文件
- **架构**: 模块化设计，独立包管理