---
name: blog-data-opt
description: 博客数据查询与操作技能。覆盖博客、待办、运动、项目、读书 5 个业务域的查询、汇总和写操作。
summary: 范围查询优先 Range 接口，批量分析优先 ExecuteCode，修改前先查 id
tools: RawGetTodosByDate,RawGetTodosRange,RawAddTodo,RawToggleTodo,RawDeleteTodo,RawUpdateTodo,RawGetExerciseByDate,RawGetExerciseRange,RawGetExerciseStats,RawAddExercise,RawToggleExercise,RawDeleteExercise,RawUpdateExercise,RawRecentExerciseRecords,RawAllBlogName,RawAllBlogNameByDate,RawAllBlogNameByDateRange,RawAllBlogNameByDateRangeCount,RawGetBlogData,RawGetBlogDataByDate,RawGetBlogByTitleMatch,RawSearchBlogContent,RawCreateBlog,RawBlogsByAuthType,RawBlogsByTag,RawGetAllBooks,RawGetBooksByStatus,RawGetReadingStats,RawUpdateReadingProgress,RawGetBookNotes,RawAddBook,RawCreateProject,RawGetProject,RawListProjects,RawUpdateProject,RawDeleteProject,RawAddProjectGoal,RawUpdateProjectGoal,RawDeleteProjectGoal,RawAddProjectOKR,RawUpdateProjectOKR,RawDeleteProjectOKR,RawUpdateProjectKeyResult,RawGetProjectSummary
agents: blog,exec_code
keywords: 博客,待办,运动,todo,exercise,blog,数据,查询,记录,周报,统计,日记,锻炼,项目,project,读书,阅读,reading,book
---

# 数据查询与操作

## 适用场景

- 查询待办、运动、博客内容或时间范围内的统计结果
- 对多天数据做聚合、对比、周报汇总
- 新增、完成、删除、更新待办或运动记录
- 创建博客或按关键词搜索博客内容

## 必须遵守

- `account` 默认使用当前用户账号，不要向用户追问
- 日期范围查询优先使用 `Range` 接口，不要循环逐天调用单日接口
- 涉及完成、删除、更新时，先查出记录 `id` 再执行修改
- 多源查询或批量统计优先使用 `ExecuteCode`，只输出最终结论

## 推荐流程

1. 先确认目标实体和时间范围。
2. 按数据类型选择接口：
   - 待办：`RawGetTodosByDate`、`RawGetTodosRange`、`RawAddTodo`、`RawToggleTodo`、`RawDeleteTodo`、`RawUpdateTodo`
   - 运动：`RawGetExerciseByDate`、`RawGetExerciseRange`、`RawGetExerciseStats`、`RawRecentExerciseRecords`、`RawAddExercise`、`RawToggleExercise`、`RawDeleteExercise`、`RawUpdateExercise`
   - 博客：`RawAllBlogName`、`RawAllBlogNameByDate`、`RawAllBlogNameByDateRange`、`RawAllBlogNameByDateRangeCount`、`RawGetBlogData`、`RawGetBlogDataByDate`、`RawGetBlogByTitleMatch`、`RawSearchBlogContent`、`RawCreateBlog`、`RawBlogsByAuthType`、`RawBlogsByTag`
   - 读书：`RawGetAllBooks`、`RawGetBooksByStatus`、`RawGetReadingStats`、`RawUpdateReadingProgress`、`RawGetBookNotes`、`RawAddBook`
   - 项目：`RawCreateProject`、`RawGetProject`、`RawListProjects`、`RawUpdateProject`、`RawDeleteProject`、`RawAddProjectGoal`、`RawUpdateProjectGoal`、`RawDeleteProjectGoal`、`RawAddProjectOKR`、`RawUpdateProjectOKR`、`RawDeleteProjectOKR`、`RawUpdateProjectKeyResult`、`RawGetProjectSummary`
3. 如果是分析任务，在 `ExecuteCode` 中完成聚合、统计、排序和格式化。
4. 如果是修改任务，先查 `id`，再调用对应的新增、更新、切换或删除接口。

## 工具选择规则

- 单日单类数据查询，直接调用单个查询工具
- 跨日期、跨类型或需要统计时，优先 `ExecuteCode + Range` 接口
- `RawCreateBlog` 适用于直接创建博客；`tags` 用 `|` 分隔，`authType` 取值为 1/2/4/8/16
- `RawAddExercise` 的 `intensity` 常见值为 `low`、`medium`、`high`
- `RawAddTodo` 的 `urgency` 和 `importance` 取值为 `1-4`

## 禁止行为

- 为了范围查询而循环逐天调单日接口
- 在需要 `id` 的修改操作前直接猜测或编造 `id`
- 让用户手动提供当前账号
- 把大段原始数据直接回灌给用户，不做整理和汇总

## 示例

- “帮我总结这周做了哪些运动”
  用 `RawGetExerciseRange` 拉区间数据，再在 `ExecuteCode` 中统计和整理
- “把今天的待办第 3 条标记完成”
  先查今天的待办列表，定位真实 `id`，再调用 `RawToggleTodo`
- “创建一篇标题是《四月计划》的博客”
  直接调用 `RawCreateBlog`，并按请求拼好 `title`、`content`、`tags`、`authType`
