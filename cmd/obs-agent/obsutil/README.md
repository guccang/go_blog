将各平台的 `obsutil` 二进制放到以下目录中：

- `obsutil/linux/obsutil`
- `obsutil/macos/obsutil`
- `obsutil/windows/obsutil.exe`

`obs-agent` 的 `/api/obs/proxy-upload` 会优先按当前操作系统查找上述路径中的程序，并执行：

```bash
obsutil cp <local-file> obs://<bucket>/<key>
```

如果需要自定义路径，也可以在 `obs-agent.json` 中配置：

```json
{
  "obsutil_path": "/absolute/path/to/obsutil",
  "obsutil_timeout_seconds": 1800
}
```
