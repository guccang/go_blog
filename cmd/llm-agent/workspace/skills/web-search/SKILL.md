---
name: web-search
description: 网络搜索技能。当需要搜索互联网获取实时信息、查询资料、获取网页内容时使用此技能。
summary: WebSearch 搜索 + WebFetch 抓取网页详情
tools: WebSearch,WebFetch
agents: blog
keywords: 搜索,search,网页,查找,天气,新闻,web,查询资料
---

# 网络搜索与网页抓取

**call_tool 使用规范见 workspace/CALL_TOOL.md**

## 使用场景

- 用户询问实时信息（天气、新闻、股价等）
- 需要查询技术文档、API 参考
- 需要获取特定网页的内容
- 用户明确要求搜索或查找网上信息

## 可用接口

| 接口 | 参数 | data 类型 |
|------|------|-----------|
| `WebSearch` | query, count(可选,默认5) | dict({query, results:[{title,url,snippet}], count}) |
| `WebFetch` | url, maxLength(可选,默认5000) | dict({url, content, length, truncated}) |

## 使用模式

### 基本搜索
直接调用 WebSearch 获取搜索结果：
```
WebSearch({"query": "Python asyncio 教程"})
```

### 搜索 + 抓取详情
先搜索获取 URL 列表，再抓取感兴趣的页面内容。

### 多关键词搜索聚合
在 ExecuteCode 中循环多个关键词批量搜索。

## 注意事项

- WebSearch 使用 Bing 搜索引擎，返回标题、URL 和摘要
- WebFetch 返回纯文本内容，HTML 标签已自动移除
- WebFetch 默认最大返回 5000 字符，可通过 maxLength 参数调整
- 仅支持 http/https 协议的 URL
- 搜索结果最多 10 条
