
## 📚 相关文档

- [编译配置说明](BUILD_GUIDE.md)
- [配置说明](SYS_CONF_GUIDE.md)


## 🔍 项目代码分析报告

基于对代码的深入分析，这是一个功能非常丰富的**Go语言博客系统**，采用了创新的"一切皆博客"设计理念，将传统博客功能与个人生产力工具深度整合。

## 🏗️ 系统架构特点

### 核心设计理念
1. **统一数据模型**: 所有功能数据都以博客格式存储在 `blogs_txt/` 目录中
2. **模块化架构**: 20+独立功能模块，每个模块都有独立的 `go.mod` 文件
3. **双重存储**: 文件系统作为主存储 + Redis作为缓存层
4. **无数据库依赖**: 纯文件存储，便于迁移和备份

### 技术栈
- **后端**: Go 1.21，模块化设计
- **前端**: HTML/CSS/JavaScript，响应式设计  
- **存储**: Markdown文件 + Redis缓存
- **加密**: AES-CBC算法
- **模板引擎**: Go template系统

## 🎯 功能模块分析

### 1. 核心博客功能 (`pkgs/blog/`)
- Markdown编写/编辑，实时预览
- 权限控制系统：public/private/encrypt/cooperation/diary
- 标签系统和全文搜索
- 加密存储敏感内容

### 2. 锻炼管理系统 (`pkgs/exercise/`)
从 `exercise.js` 代码可以看出功能极其完善：
- **锻炼记录**: 支持4种类型（有氧、力量、柔韧、运动）
- **智能计算**: 基于MET值的精确卡路里计算
- **模板管理**: 预设锻炼计划，支持批量添加
- **集合功能**: 模板组合，一键添加整套训练
- **个人档案**: 身高体重管理，个性化计算
- **统计分析**: 周/月/年度数据可视化

### 3. 任务管理系统 (`pkgs/todolist/`)
- 按日期组织的待办事项
- 时间预估和追踪
- 拖拽排序功能
- 历史完成情况统计

### 4. AI系统接入(`pkgs/llm/`)
- 接入deepseek
- 接入mcp 文件访问 redis访问
- 接入自定义mcp-server-blog，博客系统也提供了mcp服务接口，获取博客系统内容
- 通过deepseek交互，可以通过访问日记，给出建设性意见，随着博客越多，AI越懂你

### 5. 其他特色功能
- **读书管理** (`pkgs/reading/`): 书籍管理、进度追踪、读书笔记
- **年度计划** (`pkgs/yearplan/`): 年度目标、月度分解、进度追踪
- **人生倒计时** (`pkgs/lifecountdown/`): 重要日期提醒
- **统计分析** (`pkgs/statistics/`): 多维度数据分析

## 🔐 安全机制

### 权限控制
```12:15:pkgs/module/module.go
const (
	EAuthType_private = 1
	EAuthType_public  = 2
	EAuthType_encrypt = 4
	EAuthType_cooperation = 8
	EAuthType_diary   = 16  // 日记博客，需要密码保护
	EAuthType_all     = 0xffff
)
```

### 加密存储
- 基于AES-CBC算法的内容加密
- 支持组合权限设置
- 密码保护的分享功能

## 🌟 技术亮点

### 1. 创新的数据存储方式
所有功能数据都转换为博客格式存储，例如：
- 锻炼记录 → `exercise-2024-01-15` 博客
- 任务列表 → `todolist-2024-01-15` 博客
- 这种方式使数据高度统一且易于管理

### 2. 高度模块化设计
每个功能包都是独立模块，支持独立开发和测试：
```12:15:go.mod
replace module => ./pkgs/module
replace control => ./pkgs/control
replace view => ./pkgs/view
```

### 3. 丰富的前端交互
从 `exercise.js` 可以看出前端功能非常完善：
- 多视图切换
- 实时数据计算
- 拖拽操作支持
- 响应式设计

### 4. 智能计算功能
锻炼模块包含复杂的MET值计算系统，能根据用户体重、锻炼类型、强度精确计算卡路里消耗。

## 💡 项目特色

### 优势
1. **数据自主性**: 所有数据都是文本文件，用户完全掌控
2. **功能完整性**: 集博客、任务、健身、读书于一体
3. **高度可定制**: 模板系统、主题切换、快捷键支持
4. **部署简单**: 单一可执行文件，支持HTTPS
5. **扩展性强**: 模块化设计便于添加新功能
6. **接入DEEPSEEK** : 接入AI，可以实时分析博客数据,通过MCP服务器访问redis和本地文件。

### 适用场景
- 个人博客网站
- 生产力工具集成平台
- 私人数据管理系统
- 团队协作平台

## 🚀 总结

这是一个**极其优秀的个人数字生活管理系统**，不仅仅是传统博客，更是一个完整的生产力工具套件。其创新的"一切皆博客"理念、模块化架构设计、以及丰富的功能特性，使其成为一个非常有价值的开源项目，特别适合注重数据隐私和功能完整性的用户使用。

项目代码质量高，架构设计合理，功能实现完善，是学习Go语言Web开发和系统设计的优秀案例。