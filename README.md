# Relay · 渡线

Relay 在电脑和 Android 之间传输文件。桌面界面采用 Go 1.26、Wails 3、React 和 TypeScript，提供中文双栏浏览器。

## 安装和连接

从 [GitHub Actions](https://github.com/songkwon/Relay/actions/workflows/package.yml) 获取最近一次成功构建，或到 [Releases](https://github.com/songkwon/Relay/releases) 查看带版本标签的构建。

| 平台 | 架构 | 包格式 | 可用连接 |
| --- | --- | --- | --- |
| macOS 14 及以上 | ARM64 和 Intel AMD64 | ZIP 内含 `Relay.app` | USB MTP、USB ADB、Wi-Fi ADB |
| Windows | ARM64 和 AMD64 | ZIP 内含便携版 `Relay.exe` | USB ADB、Wi-Fi ADB |

macOS 包使用临时签名，尚未做 Apple 公证。Windows 版需要同架构的 Microsoft Edge WebView2 Runtime，当前为未签名便携版。

| 连接方式 | 手机上的操作 | 电脑上的依赖 |
| --- | --- | --- |
| USB MTP | 解锁手机，用数据线连接，选择「文件传输」 | macOS 版随包提供 libmtp 和 libusb |
| USB ADB | 在开发者选项里开启 USB 调试，连接后授权电脑 | Android SDK Platform-Tools |
| Wi-Fi ADB | 手机和电脑连接同一个 Wi-Fi | Android SDK Platform-Tools |

ADB 模式可使用系统 PATH、Android SDK 目录里的 `adb`，或与 Relay 可执行文件放在同一目录的 Platform-Tools。[下载 Android SDK Platform-Tools](https://developer.android.com/tools/releases/platform-tools)

- **Android 11 及以上**。打开手机的「无线调试」，进入「使用配对码配对设备」。在 Relay 选择「六位码配对」，发现附近设备或填写手机显示的配对地址，再输入六位码。配对成功后，Relay 会尝试自动连接同一部手机。手动连接时，填写「无线调试」主页面的连接地址和端口。
- **Android 10 及更早版本**。先用 USB 授权 ADB，然后在 Relay 的「旧版 Android」页切换到临时 Wi-Fi。此模式使用未加密的 ADB TCP，请在可信局域网使用。结束后点击「关闭临时 Wi-Fi」以停止手机的监听。如果关闭时手机离线，请插回 USB 重试，或在手机上关闭 USB 调试。

Relay 保留当前设备和所在目录。USB 或 Wi-Fi 暂时断开时，同一设备恢复后会重新读取目录。切换到其他设备请使用「设备连接」。若 USB MTP 无法打开，可退出其他正在访问手机的文件工具，或在已授权 USB 调试时选择 ADB。

在双栏中分别打开源目录和目标目录，勾选文件或文件夹，再用底部按钮传输。也可以把本机项目拖到 Android 目录。遇到同名项目会报告错误，结果可在「传输记录」查看。macOS 可将设备只读挂载到 Finder，写入操作在 Relay 中进行。

## 源码构建

需要 Go 1.26.10、Node.js 24 和 npm。Windows 使用 PowerShell 7.4 或更新版本。

```sh
# macOS，需 Xcode 命令行工具和 pkg-config
./scripts/build-macos-deps.sh
./scripts/package-macos.sh
```

```powershell
# Windows，-Architecture 可选 amd64 或 arm64
pwsh -File scripts/package-windows.ps1 -Architecture amd64
```

产物在 `build/`。macOS 可以用 `RELAY_CODESIGN_IDENTITY` 指定签名证书。CI 在 `main` 更新、PR 或手动运行时上传四个目标的 ZIP。推送形如 `v0.2.0` 的标签会在四个目标成功后发布 GitHub Release 和 SHA-256 校验文件。

## 组件和许可证

操作通知采用 [Rare UI Toast](https://www.rareui.com/components/toast)。其许可证位于 `frontend/src/rare-ui/LICENSE`，并随应用包分发。原生依赖见 `scripts/native-notices.txt`。
