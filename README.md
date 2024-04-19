# 如何使用 
 此链接为go_blog搭建的私人博客,访问此页面,有具体的版本介绍以及如何部署和使用。
 http://blog.guccang.cn/tag?tag=blog
---
# 功能列表

## 1. 登录 **OK**
## 2. 权限模块 **OK**
 > 1. 通过配置文本文件配置用户和密码
## 3. AddPage页面 **OK**
 > 添加博客
## 4. ShowPage页面 **OK**
 > 查看原有博客
## 5. MainPage页面 **OK**
 > 浏览所有博客，根据关键字搜索博客
## 6. AddPage显示编辑框和markdown预览框 **OK**
## 7. ShowPage显示默认显示markdown预览模式，可点击右上角编辑按钮,页面同时显示编辑框和预览框 **OK**
## 8. 手机适配  **OK**
   > 1. 手机屏幕太小需要适配不同页面
  2. AddPage默认显示编辑框,点击按钮可已切换到预览框。（pc端可同时显示）
  3. ShowPage默认显示预览,点击编辑后只显示编辑框（pc端可同时显示）
   
## 9. 深夜模式, 时间超过18:55分切换页面主题为深色 <span style="color: red;"> **DOING** </span>
## 10. 搜索功能,根据文本匹配博客名字和博客内容 （忽略大小写） **OK**
## 11. 展示博客页面, 显示当前所有博客, 按照修改时间降序排列。 **OK**
## 12. 增加cookie功能 **OK**
## 13. 文件服务模块增加权限控制    **OK**
## 14. 20240202支持分享博客到外部  **OK**
  1. 问题开启次功能后13有问题关闭了 
## 15. 20240202增加博客的访问控制修改,private public  **OK**

## 16. 20240204搜索框扩展支持private public 分类搜索 **OK**
  1. $private 显示所有private博客
  2. $public  显示所有public博客
  3. $private hello 显示所有包含hello的private 博客
  4. $public hello 显示所有包含hello的public博客

## 17. 增加主页search input自动补全$private $public **OK**

## 18. 增加tags功能，用于博客分类。blog增加，搜索增加  **OK**

## 19. 增加评论comment功能 **OK**

## 20. 增加打印反向代理的ip日志 **OK**

## 21. 为每日任务增加邮件提醒通知，未完成的任务邮件通知，每小时一次通知。<span style="color: red;"> **DOING** </span>

## 22. 设置登录页面元素居中显示 **OK**

## 23. html textarea支持vim编辑 **OK**
   1. https://github.com/toplan/Vim.js
   2. 上述只支持部分vim快捷键，并且支持的不完整，但是可用

## 24. 支持email功能<span style="color: red;">  **DOING** </span>
   1. public 被访问时, 邮件通知
   2. 每天统计blog访问量，发送到邮箱中去

## 25. 所有配置都是用blog实现。重启也是用search窗口实现。重新加载配置文件也是。<span style="color: red;"> **DOING** </span>

## 26. 增加按照时间查找blog的功能 @time c/m/a 正则表达式时间  创建修改访问时间的blog <span style="color: red;"> **DOING** </span>

## 27. 增加blog加密,使用aes-cbc加密 **OK**

## 28. 增加二次确认修改提示 **OK**

## 29. 增加删除功能  **OK**

## 30. 增加模版功能,每日任务，锻炼等等模版  **OK**

## 31. blog数量主页限制为100, 现在越来越多的blog, 没必要显示所有blog。可以通过搜索框显示全部blog **OK**

## 32. 支持最多2个端登陆，FIFO模型删除session **OK**

## 33. 数据可视化支持 <span style="color: red;"> **DOING** </span>

## 34. 标签展示  **OK** 根据tag对外展示搜tag的blog，方便系统性的展示某写内容 **OK**

## 35. 标签替换 @tag from to 将标签从from替换为to,方便整体替换删除标签。to为空，删除from标签 **OK**

## 36. 增加@main标签  直接跳转到主页 /link **OK**

## 37. searchButton响应键盘回车事件，不用鼠标点击了，search框输入完成后直接敲击回车完成搜索。 **OK**

## 38. 增加监听CTR+LEFT/RIGHT按键，用于返回上一历史页面。**OK**
---
## 问题
1. 20240204增加分享博客到外部后，导致13权限控制问题，导致分享的博客应用的数据呗权限系统拦截，目前没有优雅的解决方案。
