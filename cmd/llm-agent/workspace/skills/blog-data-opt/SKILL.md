---
name: blog-data-opt
description: 博客数据查询与操作技能。查询待办、运动、博客等数据并做汇总分析，以及新增/完成/删除运动记录、新增/完成/删除待办、创建博客等数据操作。
summary: 用 Range 接口批量查询，操作前先查 id
tools: RawGetTodosByDate,RawGetTodosRange,RawAddTodo,RawToggleTodo,RawDeleteTodo,RawUpdateTodo,RawGetExerciseByDate,RawGetExerciseRange,RawGetExerciseStats,RawAddExercise,RawToggleExercise,RawDeleteExercise,RawUpdateExercise,RawRecentExerciseRecords,RawAllBlogNameByDate,RawAllBlogNameByDateRange,RawAllBlogNameByDateRangeCount,RawGetCurrentTaskByRageDate,RawGetBlogData,RawSearchBlogContent,RawCurrentDate,RawCreateBlog
keywords: 博客,待办,运动,todo,exercise,blog,数据,查询,记录,周报,统计,日记,锻炼
---

# 数据查询与操作

**call_tool 使用规范见 workspace/CALL_TOOL.md**

## 查询原则

- 多源数据查询优先使用 ExecuteCode 批量处理，减少工具调用轮次
- account 使用当前用户账号，不要向用户询问
- **日期范围查询优先使用 Range 接口**，不要循环逐天调用单日接口

## 可用的查询接口

### 基础工具
| 接口 | 参数 | data 类型 |
|------|------|-----------|
| `RawCurrentDate` | (无需参数) | str，如 `"2026-03-24"` |

### 单日查询
| 接口 | 参数 | data 类型 |
|------|------|-----------|
| `RawGetTodosByDate` | account, date | list，每项含 id/content/done |
| `RawGetExerciseByDate` | account, date | list，每项含 id/name/duration 等 |
| `RawAllBlogNameByDate` | account, date | str(空格分隔的标题列表) |

### 范围查询
| 接口 | 参数 | data 类型 |
|------|------|-----------|
| `RawGetTodosRange` | account, startDate, endDate | dict, key 为日期 |
| `RawGetExerciseRange` | account, startDate, endDate | list |
| `RawAllBlogNameByDateRange` | account, startDate, endDate | str(空格分隔的标题列表) |
| `RawAllBlogNameByDateRangeCount` | account, startDate, endDate | str(数字) |
| `RawGetCurrentTaskByRageDate` | account, startDate, endDate | dict, key 为日期 |
| `RawGetExerciseStats` | account, days | str(格式化文本) |
| `RawRecentExerciseRecords` | account, days | str(格式化列表) |

### 博客内容查询
| 接口 | 参数 | data 类型 |
|------|------|-----------|
| `RawGetBlogData` | account, title | str(markdown 纯文本) |
| `RawSearchBlogContent` | account, keyword | str(格式化搜索结果) |

## 可用的操作接口

### 待办操作
| 接口 | 参数 | data 类型 |
|------|------|-----------|
| `RawAddTodo` | account, date, content, hours, minutes, urgency, importance | dict(新建的待办项) |
| `RawToggleTodo` | account, date, id | dict({success:true}) |
| `RawDeleteTodo` | account, date, id | dict({success:true}) |
| `RawUpdateTodo` | account, date, id, hours, minutes | dict({success:true}) |

### 运动记录操作
| 接口 | 参数 | data 类型 |
|------|------|-----------|
| `RawAddExercise` | account, date, name, exerciseType, duration, intensity, calories, notes | dict(新建的运动记录) |
| `RawToggleExercise` | account, date, id | dict({success:true}) |
| `RawDeleteExercise` | account, date, id | dict({success:true}) |
| `RawUpdateExercise` | account, date, id, name, exerciseType, duration, intensity, calories, notes | dict({success:true}) |

### 博客操作
| 接口 | 参数 | data 类型 |
|------|------|-----------|
| `RawCreateBlog` | account, title, content, tags, authType, encrypt(可选) | str(操作结果) |

- `tags`: 多个标签用 `|` 分隔
- `authType`: 1=私有, 2=公开, 4=加密, 8=协作, 16=日记
- `encrypt`: 0=否, 1=是（可选）
- 创建成功后返回博客链接，格式为 `[title](/get?blogname=title)`

## 操作注意事项

- 完成/删除操作需要先查询获取记录的 `id`，再调用对应接口
- `RawToggleExercise` 切换完成状态（未完成→完成，完成→未完成）
- `RawAddExercise` 的 `exerciseType` 常见值：跑步、游泳、力量训练、瑜伽、骑行等
- `RawAddExercise` 的 `intensity` 可选值：low、medium、high
- `RawAddTodo` 的 urgency/importance 取值 1-4（1最高）

## 常见查询模式

### 单日数据
直接调用对应的单日查询工具即可。

### 日期范围数据
使用 ExecuteCode + Range 接口一次性获取，不要循环逐天调用。

### 数据聚合分析
查询完数据后在 Python 中完成统计、汇总、对比等分析工作，只 print 最终结论。
