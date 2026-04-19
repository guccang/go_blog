# OBS 流程与华为云 SDK 使用说明

本文档说明当前仓库中 OBS 相关的整体架构、上传下载链路、关键配置，以及华为云 OBS Go SDK 在代码中的具体使用方式。

适用范围：

- `cmd/common/obsstore/`
- `cmd/obs-agent/`
- `cmd/app-agent/`
- `cmd/flutter-client-for-appagent/flutter_client_for_appagent/`

## 1. 目标与职责划分

当前方案的目标是：

- APK 和其他大附件落到华为云 OBS
- 客户端不直接与 `obs-agent` 交互
- `app-agent` 作为客户端唯一入口
- `obs-agent` 作为内部 OBS 能力代理
- 最终下载流量尽量直达云厂商 OBS，避免业务服务转发大文件

各模块职责如下：

- `obsstore`
  封装华为云 OBS Go SDK，提供统一的对象存储能力。
- `obs-agent`
  负责签名下载 URL、签名上传 URL、代理上传、列目录、删对象、查对象信息。
- `app-agent`
  负责业务侧附件上传、APK 下发、下载票据签发，以及对 `obs-agent` 的内部调用。
- Flutter 客户端
  只与 `app-agent` 交互，不直接访问 `obs-agent`。

## 2. 当前链路总览

### 2.1 APK 上传链路

当前 APK 上传链路如下：

1. Flutter 构建 APK，或通过脚本将 APK 提交到 `app-agent`
2. `app-agent` 接收 `multipart/form-data` 的 `/api/app/upload-apk`
3. `app-agent` 先把 APK 保存到本地附件目录
4. `app-agent` 调用内部对象存储接口
5. 当前对象存储优先走 `obs-agent /api/obs/proxy-upload`
6. `obs-agent` 再调用华为云 OBS SDK `PutObject`
7. 上传成功后，`app-agent` 在消息元数据中写入：
   - `file_id`
   - `object_key`
   - `storage_provider=obs`
   - `download_via=obs-agent`
   - `download_ticket`
8. Flutter 客户端收到 APK 消息，可点击下载安装

代码位置：

- 上传入口：[cmd/app-agent/handler.go](/Users/guccang/github_repo/go_blog/cmd/app-agent/handler.go:570)
- 本地保存 APK：[cmd/app-agent/bridge.go](/Users/guccang/github_repo/go_blog/cmd/app-agent/bridge.go:498)
- `app-agent` 通过 `obs-agent` 代理上传：[cmd/app-agent/obs_support.go](/Users/guccang/github_repo/go_blog/cmd/app-agent/obs_support.go:112)
- `obs-agent` 代理上传到 OBS：[cmd/obs-agent/handler.go](/Users/guccang/github_repo/go_blog/cmd/obs-agent/handler.go:183)

### 2.2 APK 下载链路

当前下载链路已经调整为客户端只访问 `app-agent`。

实际链路如下：

1. Flutter 客户端请求 `GET /api/app/attachments/{file_id}`
2. `app-agent` 校验登录态和用户身份
3. `app-agent` 根据 `file_id` 找到本地附件
4. 如果文件是 APK，且配置了 `obs_agent_base_url`
5. `app-agent` 使用 `download_ticket_secret` 为当前用户签发下载票据
6. `app-agent` 内部请求 `obs-agent /api/obs/download/{file_id}?ticket=...`
7. `obs-agent` 校验票据后调用 OBS SDK 生成签名 GET URL
8. `obs-agent` 返回云厂商 OBS 签名 URL
9. `app-agent` 将该 URL 以 `302 Location` 返回给客户端
10. Flutter 客户端跟随重定向，直接从华为云 OBS 下载 APK

如果第 6 到 8 步失败，则 `app-agent` 自动回退为本地 `ServeFile`。

代码位置：

- 附件下载入口：[cmd/app-agent/handler.go](/Users/guccang/github_repo/go_blog/cmd/app-agent/handler.go:423)
- 内部请求 `obs-agent` 获取下载 URL：[cmd/app-agent/handler.go](/Users/guccang/github_repo/go_blog/cmd/app-agent/handler.go:531)
- `obs-agent` 下载签名入口：[cmd/obs-agent/handler.go](/Users/guccang/github_repo/go_blog/cmd/obs-agent/handler.go:43)

## 3. 时序图

### 3.1 上传时序

```text
Flutter / push-apk.sh
        |
        | POST /api/app/upload-apk
        v
app-agent
        |
        | 保存到本地附件目录
        |
        | POST /api/obs/proxy-upload
        v
obs-agent
        |
        | OBS SDK PutObject
        v
Huawei OBS
```

### 3.2 下载时序

```text
Flutter Client
        |
        | GET /api/app/attachments/{file_id}
        v
app-agent
        |
        | Issue download ticket
        | GET /api/obs/download/{file_id}?ticket=...
        v
obs-agent
        |
        | OBS SDK CreateSignedUrl(GET)
        v
Huawei OBS
        ^
        |
        | signed GET URL
        |
app-agent
        |
        | 302 Location: https://obs....
        v
Flutter Client
        |
        | GET https://obs....
        v
Huawei OBS
```

## 4. 关键配置说明

### 4.1 `obs-agent.json`

`obs-agent` 必须直接持有华为云 OBS 配置，因为真正调用 SDK 的是它。

典型配置：

```json
{
  "http_port": 9004,
  "receive_token": "test-token",
  "download_ticket_secret": "replace-with-a-long-random-secret",
  "signed_url_ttl_seconds": 300,
  "obs": {
    "endpoint": "obs.cn-north-4.myhuaweicloud.com",
    "bucket": "obs-app-agent",
    "ak": "your-ak",
    "sk": "your-sk",
    "region": "cn-north-4",
    "key_prefix": "app-attachments",
    "path_style": false
  }
}
```

字段说明：

- `receive_token`
  保护 `obs-agent` HTTP 接口。
- `download_ticket_secret`
  用于校验附件下载票据。
- `signed_url_ttl_seconds`
  生成签名 URL 的有效期。
- `obs.endpoint/bucket/ak/sk/region`
  华为云 OBS 访问配置。
- `obs.key_prefix`
  对象统一前缀。

### 4.2 `app-agent.json`

当前 `app-agent` 不再需要直连 OBS 的 `ak/sk` 等配置。

最小配置：

```json
{
  "receive_token": "123456",
  "obs_agent_base_url": "http://blog.guccang.cn:9004",
  "obs_agent_token": "test-token",
  "download_ticket_secret": "same-as-obs-agent",
  "download_ticket_ttl_seconds": 300
}
```

字段说明：

- `obs_agent_base_url`
  `app-agent` 内部调用 `obs-agent` 的地址。
- `obs_agent_token`
  `app-agent` 调 `obs-agent` 时携带的认证 token。
- `download_ticket_secret`
  必须与 `obs-agent` 完全一致。
- `download_ticket_ttl_seconds`
  下载票据有效期。

## 5. `download_ticket_secret` 的作用

`download_ticket_secret` 不是 OBS 的 AK/SK，也不是登录 token。它只用于签发和校验“附件下载票据”。

职责拆分如下：

- `app-agent`
  使用 `download_ticket_secret` 签发 `download_ticket`
- `obs-agent`
  使用相同的 `download_ticket_secret` 校验 `download_ticket`

如果两边配置不一致，会出现：

- APK 消息已下发
- `download_ticket` 也存在
- 但 `obs-agent /api/obs/download/{file_id}` 返回 `401/403`

代码位置：

- 票据签发：[cmd/app-agent/obs_support.go](/Users/guccang/github_repo/go_blog/cmd/app-agent/obs_support.go:355)
- 票据校验：[cmd/obs-agent/handler.go](/Users/guccang/github_repo/go_blog/cmd/obs-agent/handler.go:68)

## 6. 华为云 OBS SDK 的使用方式

本仓库通过 `cmd/common/obsstore` 封装华为云 OBS Go SDK。

SDK 依赖：

- `github.com/huaweicloud/huaweicloud-sdk-go-obs/obs`

代码位置：

- [cmd/common/obsstore/store.go](/Users/guccang/github_repo/go_blog/cmd/common/obsstore/store.go:1)

### 6.1 初始化客户端

初始化时传入：

- `ak`
- `sk`
- `endpoint`
- `region`
- `path_style`
- `disable_ssl_verify`

封装代码：

```go
client, err := obs.New(
    cfg.AccessKey,
    cfg.SecretKey,
    cfg.Endpoint,
    obs.WithPathStyle(cfg.PathStyle),
    obs.WithSslVerify(!cfg.DisableSSLVerify),
    obs.WithRegion(cfg.Region),
    obs.WithConnectTimeout(10),
    obs.WithSocketTimeout(30),
)
```

位置：

- [cmd/common/obsstore/store.go](/Users/guccang/github_repo/go_blog/cmd/common/obsstore/store.go:59)

### 6.2 上传对象 `PutObject`

仓库中的上传调用方式：

```go
_, err := s.client.PutObject(&obs.PutObjectInput{
    PutObjectBasicInput: obs.PutObjectBasicInput{
        ObjectOperationInput: obs.ObjectOperationInput{
            Bucket:   s.cfg.Bucket,
            Key:      key,
            Metadata: cloneStringMap(req.Metadata),
        },
        HttpHeader: obs.HttpHeader{
            ContentType: strings.TrimSpace(req.ContentType),
        },
        ContentLength: req.Size,
    },
    Body: req.Body,
})
```

位置：

- [cmd/common/obsstore/store.go](/Users/guccang/github_repo/go_blog/cmd/common/obsstore/store.go:108)

用途：

- `obs-agent /api/obs/proxy-upload`
- `app-agent` 间接调用 `obs-agent` 上传附件

### 6.3 检查对象是否存在 `HeadObject`

用于避免重复上传：

```go
_, err := s.client.HeadObject(&obs.HeadObjectInput{
    Bucket: s.cfg.Bucket,
    Key:    key,
})
```

位置：

- [cmd/common/obsstore/store.go](/Users/guccang/github_repo/go_blog/cmd/common/obsstore/store.go:139)

用途：

- 上传前判断对象是否已经存在
- 下载前确认对象存在

### 6.4 获取对象元信息 `GetObjectMetadata`

用于查询：

- 文件大小
- Content-Type
- ETag
- 最后修改时间
- 自定义 metadata

位置：

- [cmd/common/obsstore/store.go](/Users/guccang/github_repo/go_blog/cmd/common/obsstore/store.go:158)

对应接口：

- `GET /api/obs/info`

### 6.5 列对象 `ListObjects`

用于按前缀列目录：

位置：

- [cmd/common/obsstore/store.go](/Users/guccang/github_repo/go_blog/cmd/common/obsstore/store.go:188)

对应接口：

- `GET /api/obs/list`

### 6.6 删除对象 `DeleteObject`

位置：

- [cmd/common/obsstore/store.go](/Users/guccang/github_repo/go_blog/cmd/common/obsstore/store.go:223)

对应接口：

- `POST /api/obs/delete`

### 6.7 生成签名上传 URL `CreateSignedUrl(PUT)`

仓库中用法：

```go
output, err := s.client.CreateSignedUrl(&obs.CreateSignedUrlInput{
    Method:  obs.HttpMethodPut,
    Bucket:  s.cfg.Bucket,
    Key:     key,
    Expires: int(ttl.Seconds()),
    Headers: map[string]string{"Content-Type": ct},
})
```

位置：

- [cmd/common/obsstore/store.go](/Users/guccang/github_repo/go_blog/cmd/common/obsstore/store.go:241)

对应接口：

- `POST /api/obs/upload`

### 6.8 生成签名下载 URL `CreateSignedUrl(GET)`

仓库中用法：

```go
output, err := s.client.CreateSignedUrl(&obs.CreateSignedUrlInput{
    Method:  obs.HttpMethodGet,
    Bucket:  s.cfg.Bucket,
    Key:     key,
    Expires: int(ttl.Seconds()),
})
```

位置：

- [cmd/common/obsstore/store.go](/Users/guccang/github_repo/go_blog/cmd/common/obsstore/store.go:280)

对应接口：

- `GET /api/obs/download/{file_id}`

## 7. 对象 Key 规则

当前附件对象 key 使用如下规则：

```text
app/{message_type}/{owner}/{canonical_file_id}/{file_name}
```

生成位置：

- [cmd/app-agent/obs_support.go](/Users/guccang/github_repo/go_blog/cmd/app-agent/obs_support.go:376)

示例：

```text
app/file/alice/YWxpY2UvYXBwLXJlbGVhc2UuYXBr/app-release.apk
```

如果配置了 `key_prefix=app-attachments`，则最终 OBS 中的对象 key 可能是：

```text
app-attachments/app/file/alice/YWxpY2UvYXBwLXJlbGVhc2UuYXBr/app-release.apk
```

## 8. 关键接口说明

### 8.1 `app-agent`

- `POST /api/app/upload-apk`
  上传并下发 APK。
- `GET /api/app/attachments/{file_id}`
  客户端统一下载入口。APK 场景优先 `302` 到 OBS。

### 8.2 `obs-agent`

- `GET /health`
  健康检查。
- `POST /api/obs/upload`
  生成签名 PUT URL。
- `POST /api/obs/proxy-upload`
  服务器代传文件到 OBS。
- `GET /api/obs/download/{file_id}`
  校验下载票据并返回签名 GET URL。
- `GET /api/obs/list`
  列对象。
- `GET /api/obs/info`
  查看对象元信息。
- `POST /api/obs/delete`
  删除对象。

## 9. 测试方法

### 9.1 `obs-agent` 自测

脚本位置：

- [cmd/obs-agent/test_obs_api.sh](/Users/guccang/github_repo/go_blog/cmd/obs-agent/test_obs_api.sh:1)

运行：

```bash
cd cmd/obs-agent
./test_obs_api.sh http://localhost:9004 test-token
```

### 9.2 `app-agent` 测试

运行：

```bash
cd cmd/app-agent
go test ./...
```

重点覆盖：

- APK 上传后写入 OBS 元数据
- 复用已有 OBS 对象
- 附件下载走 `302` 重定向
- `obs-agent` 失败时回退本地文件

### 9.3 Flutter 下载测试

运行：

```bash
cd cmd/flutter-client-for-appagent/flutter_client_for_appagent
flutter test test/download_attachment_test.dart
```

覆盖：

- 断点续传
- 服务端重定向下载
- 无重定向时直接从 `app-agent` 下载

## 10. 常见问题

### 10.1 为什么 `app-agent` 不再需要 OBS 的 `ak/sk`？

因为当前设计中，真正调用华为云 OBS SDK 的服务是 `obs-agent`。`app-agent` 只需要知道：

- `obs_agent_base_url`
- `obs_agent_token`
- `download_ticket_secret`

### 10.2 为什么客户端不直接访问 `obs-agent`？

当前方案已经调整为客户端只访问 `app-agent`，原因是：

- 统一业务入口
- 避免客户端感知内部服务
- 方便权限、会话、审计集中到 `app-agent`

### 10.3 `302` 后客户端下载的是谁？

客户端下载的是华为云 OBS 的签名 URL，不是 `obs-agent` 本身。

### 10.4 如果 `obs-agent` 签名失败怎么办？

当前 `app-agent` 会回退到本地 `ServeFile`，保证 APK 仍然可下载。

## 11. 建议

- `download_ticket_secret` 使用长随机字符串，且 `app-agent` 与 `obs-agent` 必须一致
- `obs-agent` 的 `receive_token` 和 `app-agent` 的 `obs_agent_token` 必须一致
- `obs-agent` 独立持有 OBS `ak/sk`，不要再复制到 `app-agent`
- `signed_url_ttl_seconds` 不要设置过长，通常 `300` 秒足够
- APK 这类大文件优先使用 `302` 到 OBS，而不是业务服务流式代理

