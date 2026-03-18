---
name: web-search
description: 网络搜索技能。当需要搜索互联网获取实时信息、查询资料、获取网页内容时使用此技能。
tools: WebSearch,WebFetch
keywords: 搜索,search,网页,查找,天气,新闻,web,查询资料
---

# 网络搜索与网页抓取

## 使用场景

- 用户询问实时信息（天气、新闻、股价等）
- 需要查询技术文档、API 参考
- 需要获取特定网页的内容
- 用户明确要求搜索或查找网上信息

## 可用接口

| 接口 | 参数 | 返回值 |
|------|------|--------|
| `WebSearch` | query, count(可选,默认5) | JSON({query, results:[{title,url,snippet}], count}) |
| `WebFetch` | url, maxLength(可选,默认5000) | JSON({url, content, length, truncated}) |

## 使用模式

### 基本搜索
直接调用 WebSearch 获取搜索结果：
```
WebSearch({"query": "Python asyncio 教程"})
```

### 搜索 + 抓取详情
先搜索获取 URL 列表，再抓取感兴趣的页面内容：
```python
# 搜索
results = call_tool("WebSearch", {"query": "Go 1.22 新特性", "count": 3})
print(results)

# 抓取第一个结果的详细内容
page = call_tool("WebFetch", {"url": "https://example.com/article", "maxLength": 8000})
print(page)
```

### 多关键词搜索聚合
```python
queries = ["Python 3.12 新特性", "Python 3.12 性能改进"]
for q in queries:
    result = call_tool("WebSearch", {"query": q, "count": 3})
    print(f"=== {q} ===")
    print(result)
```

## 注意事项

- WebSearch 使用 Bing 搜索引擎，返回标题、URL 和摘要
- WebFetch 返回纯文本内容，HTML 标签已自动移除
- WebFetch 默认最大返回 5000 字符，可通过 maxLength 参数调整
- 仅支持 http/https 协议的 URL
- 搜索结果最多 10 条
