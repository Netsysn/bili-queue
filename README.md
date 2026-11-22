<div align="center">
  <img src="assets/biliqueue-icon.png" width="96" alt="BiliQueue 图标">

  # BiliQueue

  **B 站直播弹幕排队助手**

  一个放在桌面角落的悬浮窗，帮主播从弹幕中整理观众的排队请求。

  [下载安装](https://github.com/Netsysn/bili-queue/releases) · [使用说明](#开始使用) · [自行编译](#自行编译) · [反馈问题](https://github.com/Netsysn/bili-queue/issues)

  ![Windows](https://img.shields.io/badge/平台-Windows-0078D4?style=flat-square)
  ![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go&logoColor=white)
  ![Wails](https://img.shields.io/badge/Wails-v3-DF0000?style=flat-square)
  ![License](https://img.shields.io/badge/许可证-MIT-green?style=flat-square)
</div>

## 界面预览

<div align="center">
  <img src="screenshot.png" alt="BiliQueue 弹幕悬浮窗界面" width="373">
</div>

## 能做什么

- **弹幕排队**：监听直播间弹幕，根据配置的关键词识别排队请求和服务器。
- **队列管理**：完成、跳过或移除队列中的观众，也可以恢复超时排队。
- **礼物入队**：设置指定礼物，用于礼物入队或付费模式。
- **悬浮窗显示**：查看弹幕、头像、粉丝勋章和礼物信息，支持深浅主题及透明度设置。

## 开始使用

1. 前往 [Releases](https://github.com/Netsysn/bili-queue/releases) 下载 `BiliQueue.exe` 并运行。
2. 打开设置，填写直播间房间号，按需调整排队关键词、服务器和超时时间。
3. 保存设置并重启程序。观众在弹幕中发送符合关键词的请求后，即可在队列中查看。

> 需要 Windows 和 Microsoft Edge WebView2。设置保存在程序目录下的 `config.json` 中。

### 快捷键

| 快捷键 | 操作 |
| :--- | :--- |
| `Ctrl + E` | 完成当前排队 |
| `Ctrl + S` | 跳过当前排队 |
| `Ctrl + R` | 恢复超时排队 |

## 自行编译

准备 Go 1.25 或更新版本、Node.js 20 或更新版本，然后在 PowerShell 中运行：

```powershell
git clone https://github.com/Netsysn/bili-queue.git
cd bili-queue

cd frontend
npm ci
npm run build
cd ..

go build -o BiliQueue.exe .
```

项目使用 Go、Wails v3 和原生 JavaScript；实时弹幕通过 WebSocket 获取，并使用 HTTP 轮询作为补充。

## 隐私与安全

程序不会读取浏览器 Cookie。手动填写的 Cookie 和身份码会以明文保存在本地配置文件中；身份码模式会向第三方签名服务发送请求。使用前请阅读 [安全说明](SECURITY.md)。

## 许可证

[MIT](LICENSE) © netsysn
