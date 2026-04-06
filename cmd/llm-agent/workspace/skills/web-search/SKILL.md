---
name: web-search
description: 网络搜索技能。当需要搜索互联网获取实时信息、查询资料、获取网页内容时，使用 llm-agent 的内置 WebSearch/WebFetch 工具。
summary: 先 WebSearch 发现候选结果，再用 WebFetch 抓取高价值页面
tools: WebSearch,WebFetch
keywords: 搜索,search,网页,查找,天气,新闻,web,查询资料
---

# 网络搜索与网页抓取

**call_tool 使用规范见 workspace/CALL_TOOL.md**

## 适用场景

- 用户询问实时信息（天气、新闻、股价等）
- 需要查询技术文档、API 参考
- 需要获取特定网页的内容
- 用户明确要求搜索或查找网上信息

## 必须遵守

- 用户明确要求搜索、查找、验证最新信息时，必须实际调用搜索工具
- 先搜索再抓取，不要一上来抓大量网页
- 汇总时优先使用可信来源，并在回答里带上来源标题或链接
- `WebFetch` 抓到的是正文纯文本，需要自行整理，不要整段转贴给用户

## 推荐流程

1. 先用 `WebSearch` 搜关键词，拿到标题、URL 和摘要。
2. 选择最相关、最可信的 1 到 3 个结果。
3. 对重点结果使用 `WebFetch` 抓取正文内容。
4. 综合搜索结果和正文内容给出答案，必要时标注来源。

## 工具选择规则

- `WebSearch`：用于发现候选网页，参数为 `query` 和可选 `count`
- `WebFetch`：用于抓取指定 URL 的正文，参数为 `url` 和可选 `maxLength`
- 用户已经给出明确 URL 时，可以直接 `WebFetch`
- 搜索结果太多时，优先抓最相关的少量页面，而不是全量抓取

## 禁止行为

- 在需要实时信息时只靠记忆作答
- 对每个搜索结果都执行 `WebFetch`，造成无效开销
- 把超长网页正文原样贴回给用户
- 对不支持的协议或明显无关页面继续抓取

## 示例

```text
WebSearch({"query": "Python asyncio 教程"})
```

先搜索，再对命中的官方文档或高质量教程使用 `WebFetch` 获取细节。
