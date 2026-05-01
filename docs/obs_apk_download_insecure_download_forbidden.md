# OBS APK 下载 `InsecureDownloadForbidden` 问题记录

本文档记录 Flutter 客户端下载 APK 时出现 `400 InsecureDownloadForbidden` 的问题现状、已确认根因、当前临时处理，以及后续待办。

适用范围：

- `cmd/app-agent/`
- `cmd/obs-agent/`
- `cmd/common/obsstore/`
- `cmd/flutter-client-for-appagent/flutter_client_for_appagent/`

## 1. 问题现象

在 2026-05-01 排查时，服务端日志显示：

- `obs-agent /api/obs/info` 查询对象成功
- `obs-agent /api/obs/download/{file_id}` 签发下载 URL 成功
- Flutter 客户端下载 APK 时仍然报错：`400 InsecureDownloadForbidden`

典型日志：

```text
2026/05/01 08:54:26 [obs-agent] info request key=app/file/ztt/.../app-release-1.0.69.apk remote=127.0.0.1:47226
2026/05/01 08:54:26 [obs-agent] info ok key=app-attachments/app/file/ztt/.../app-release-1.0.69.apk size=89738717 type=application/vnd.android.package-archive
2026/05/01 08:54:27 [obs-agent] download request file_id=enR0LzIwMjYwNTAxL2FwcC1yZWxlYXNlLTEuMC42OS5hcGs remote=127.0.0.1:47230
2026/05/01 08:54:27 [obs-agent] download ok file_id=enR0LzIwMjYwNTAxL2FwcC1yZWxlYXNlLTEuMC42OS5hcGs key=app/file/ztt/.../app-release-1.0.69.apk ttl=300s
```

结论：问题不在 `obs-agent` 是否签名成功，而在“签名 URL 指向的访问域名类型”。

## 2. 已确认根因

华为云 OBS 官方文档明确说明：

- 通过桶默认域名下载 `.apk` 或 `.ipa` 文件，会返回 `400 InsecureDownloadForbidden`
- 通过桶自定义域名访问 `.apk` 或 `.ipa` 文件，不受该限制

官方文档：

- `通过自定义域名访问桶`
  https://support.huaweicloud.com/usermanual-obs/obs_03_0032.html
- `下载对象`
  https://support.huaweicloud.com/usermanual-obs/obs_03_0317.html
- `设置桶的自定义域名`
  https://support.huaweicloud.com/api-obs/obs_04_0059.html

要点摘录：

- 桶默认域名下载 `.apk/.ipa` 会触发 `InsecureDownloadForbidden`
- 自定义域名需要绑定到桶
- 中国区使用自定义域名前通常需要完成 ICP 备案
- 如需 `HTTPS`，还需要为该自定义域名配置证书

## 3. 当前系统中的实际触发方式

当前链路为：

1. Flutter 客户端请求 `app-agent` 附件下载接口
2. `app-agent` 请求 `obs-agent` 生成签名下载 URL
3. `obs-agent` 返回华为云 OBS 签名 URL
4. `app-agent` 302 重定向到该 URL
5. Flutter 客户端跟随重定向下载 APK

如果该签名 URL 的主机仍然是类似下面的 OBS 默认域名：

```text
https://bucket-name.obs.cn-north-4.myhuaweicloud.com/...
```

则客户端实际访问 OBS 时，会被华为云按 `.apk` 后缀规则拦截，返回：

```text
400 InsecureDownloadForbidden
```

## 4. 当前已做的代码侧临时处理

为避免客户端继续拿到一个注定失败的 OBS 默认域名 APK 下载地址，当前代码已增加兜底策略：

- 当 `app-agent` 检测到 `obs-agent` 返回的 APK 下载地址仍然是华为 OBS 默认域名时
- 不再对客户端返回 302
- 直接回退为 `app-agent` 本地文件下载

这样做的目的：

- 先恢复 Flutter 客户端 APK 下载能力
- 避免用户继续遇到 `InsecureDownloadForbidden`
- 将“自定义域名配置”从阻塞项降级为后续优化项

相关代码位置：

- 回退判定与下载 URL 处理：
  [cmd/app-agent/handler.go](/Users/guccang/github_repo/go_blog/cmd/app-agent/handler.go:541)
- OBS 公网下载域名支持：
  [cmd/common/obsstore/store.go](/Users/guccang/github_repo/go_blog/cmd/common/obsstore/store.go:17)
- OBS 示例配置：
  [cmd/obs-agent/obs-agent.json.example](/Users/guccang/github_repo/go_blog/cmd/obs-agent/obs-agent.json.example:1)

## 5. 后续正式方案

正式方案不是修改 Flutter，也不是依赖本地回退，而是给 OBS 桶配置“可公网访问的自定义下载域名”。

建议流程：

1. 准备一个自己可控的公网子域名
   例如：`download.example.com`
2. 在华为云 OBS 控制台为目标桶绑定该自定义域名
3. 按 OBS 控制台要求，在 DNS 服务商处添加 `CNAME`
4. 将该域名解析到 OBS 提供的目标域名
5. 如需 `HTTPS`，为该域名配置证书
6. 在 `obs-agent` 配置中填写：

```json
{
  "obs": {
    "endpoint": "obs.cn-north-4.myhuaweicloud.com",
    "public_endpoint": "https://download.example.com"
  }
}
```

配置完成后，`obs-agent` 生成签名 URL 时将优先改写为该公网自定义域名，客户端即可继续走 OBS 直链下载。

## 6. 关于 “任意自定义域名是否都可以”

不可以。至少需要满足：

- 域名归己方控制，可修改 DNS
- 该域名已绑定到目标 OBS 桶
- 中国区通常已完成 ICP 备案
- 如需 `HTTPS`，证书已正确配置
- 域名不能只是“看起来像一个下载域名”，必须真实完成 OBS 绑定与 DNS 配置

## 7. 关于 “上传时改后缀绕过限制” 的讨论

曾讨论过一种规避思路：

- 上传到 OBS 时不使用 `.apk` 后缀
- 例如对象名改为 `.apk.obs`
- Flutter 下载完成后再重命名为 `.apk`

当前结论：

- 从文档表述看，OBS 的限制明确针对“后缀为 `.apk` 或 `.ipa` 的对象”
- 因此这种方式“理论上可能暂时绕过当前规则”
- 但这不是官方支持方案，不建议作为正式方案

原因：

- 依赖平台规则细节，后续可能失效
- 对象名与真实文件类型不一致，增加排障和维护复杂度
- 后续若接入 CDN、浏览器下载、分享链路、第三方客户端，风险更高
- 会把下载成功建立在“规避规则”而不是“符合官方接入方式”上

结论：

- 可以作为最后手段的临时规避思路保留
- 不作为当前推荐实施方案

## 8. 当前建议

短期：

- 保持现有本地回退逻辑，优先恢复 APK 下载可用性

中期：

- 配置 OBS 自定义公网下载域名
- 在 `obs-agent` 中启用 `obs.public_endpoint`

长期：

- 让 APK 下载默认走 OBS 自定义域名直链
- 将 `app-agent` 本地回退仅保留为灾备兜底

## 9. 待办

- [ ] 确认是否已有可用于下载的自定义公网域名
- [ ] 确认域名是否具备中国区 ICP 备案条件
- [ ] 在 OBS 控制台完成桶自定义域名绑定
- [ ] 在 DNS 服务商处完成 `CNAME` 配置
- [ ] 如需 HTTPS，配置证书
- [ ] 更新 `obs-agent` 生产配置中的 `obs.public_endpoint`
- [ ] 联调 Flutter APK 下载链路，验证已不再触发 `InsecureDownloadForbidden`
