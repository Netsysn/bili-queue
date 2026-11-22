# BiliQueue

一个用于 B 站直播的 Windows 排队悬浮窗。它会监听直播间弹幕，识别观众的排队请求，并在桌面上显示队列。

![BiliQueue 截图](screenshot.png)

## 功能

- 监听弹幕，按自定义关键词识别排队请求和服务器。
- 查看和管理队列，支持完成、跳过、移除及恢复超时排队。
- 显示弹幕、头像、粉丝勋章和礼物信息；可设置指定礼物入队。
- 支持深色、浅色主题和窗口透明度设置。

## 使用

1. 从 [Releases](https://github.com/netsysn/bili-queue/releases) 下载并运行 `BiliQueue.exe`。
2. 在设置中填写直播间房间号，按需调整排队关键词、服务器和超时时间。
3. 保存设置并重启程序，回到直播间即可使用。

| 快捷键 | 操作 |
| --- | --- |
| `Ctrl+E` | 完成当前排队 |
| `Ctrl+S` | 跳过当前排队 |
| `Ctrl+R` | 恢复超时排队 |

程序需要 Windows 和 Microsoft Edge WebView2。配置保存在程序所在目录的 `config.json` 中。

## 自行编译

需要 Go 1.25 或更新版本、Node.js 20 或更新版本。

```powershell
git clone https://github.com/netsysn/bili-queue.git
cd bili-queue

cd frontend
npm ci
npm run build
cd ..

go build -o BiliQueue.exe .
```

项目使用 Go、Wails v3 和原生 JavaScript。实时弹幕通过 WebSocket 获取，并使用 HTTP 轮询作为补充。

## 隐私

程序不会读取浏览器 Cookie。手动填写的 Cookie 和身份码会以明文保存在本地配置文件中。身份码模式会向第三方签名服务发送请求；使用前请阅读 [安全说明](SECURITY.md)。

## 许可证

[MIT](LICENSE) © netsysn
