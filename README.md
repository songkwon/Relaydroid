# Relaydroid

用 USB 或 Wi-Fi 连接 Android 手机和 macOS 或 Windows 电脑，在双栏界面中浏览和传输文件。

## 下载与安装

### 通过 Homebrew 安装（macOS）

支持 macOS 14 及以上。Homebrew 会自动选择 Apple Silicon 或 Intel 版本。当前版本的 Mac 安装包已签名并通过 Apple 公证。

```sh
brew tap songkwon/relaydroid https://github.com/songkwon/relaydroid.git
brew install --cask songkwon/relaydroid/relaydroid
```

已安装的用户可以这样升级。

```sh
brew update
brew upgrade --cask songkwon/relaydroid/relaydroid
```

### 手动下载

打开 [GitHub Releases](https://github.com/songkwon/relaydroid/releases/latest)，按电脑的系统和芯片下载一个 ZIP。

| 系统和芯片 | 下载文件 |
| --- | --- |
| macOS · Apple Silicon | `Relaydroid-macOS-arm64.zip` |
| macOS · Intel | `Relaydroid-macOS-amd64.zip` |
| Windows · Intel 或 AMD | `Relaydroid-windows-amd64.zip` |
| Windows · ARM | `Relaydroid-windows-arm64.zip` |

在 Mac 上解压后，将 `Relaydroid.app` 拖到「应用程序」。在 Windows 上解压后运行 `Relaydroid.exe`。Windows 提供未签名的便携版，需要对应架构的 Microsoft Edge WebView2 Runtime。

`libmtp` 和 `libusb` 的源码附件供查阅，`SHA256SUMS.txt` 供手动校验。安装只需上表中的一个 ZIP。

## 连接 Android

### USB 文件传输（macOS）

解锁手机，用数据线连接 Mac，然后在手机上选择「文件传输」。Mac 安装包已内置 MTP 所需的库。如果设备无法打开，可先退出其他正在访问手机的文件传输工具。

### USB 调试（macOS、Windows）

在 Android 开发者选项中开启「USB 调试」，插入数据线后在手机上授权这台电脑。USB 调试和 Wi-Fi 连接均使用 [Android SDK Platform-Tools](https://developer.android.com/tools/releases/platform-tools) 中的 `adb`。Relaydroid 可从系统 PATH、Android SDK 或应用可执行文件所在目录找到它。

### Wi-Fi（macOS、Windows）

把手机和电脑接入同一个 Wi-Fi，然后在 Relaydroid 的「设备连接」里打开 Wi-Fi 连接。

- **Android 11 及以上**。在手机上打开「无线调试 → 使用配对码配对设备」。在 Relaydroid 选择「六位码配对」，选择发现的设备，或手动填写手机显示的配对地址，再输入六位码。配对成功后会尝试自动连接。手动连接时，使用手机「无线调试」主页面上的地址和端口。
- **Android 10 及更早版本**。先通过 USB 调试授权，再打开「旧版 Android」并选择「切换到临时 Wi-Fi」。这个模式使用未加密的 ADB 连接，请在可信局域网中使用。结束后点「关闭临时 Wi-Fi」。手机离线时，可插回 USB 重试，也可以在手机上关闭 USB 调试。

## 传输文件

在双栏中打开源目录和目标目录，选中要传的文件或文件夹，再点底部的传输按钮。也可以把电脑上的文件直接拖到 Android 目录。同名文件会报告冲突，详情可在「传输记录」查看。

设备暂时断开后，重新连上同一部手机会恢复到先前的目录。需要换手机时，从「设备连接」切换。macOS 还支持将设备只读挂载到 Finder，写入文件请使用 Relaydroid。

## 从源码构建

需要 Go 1.26.10、Node.js 24 和 npm。构建产物位于 `build/`。

**macOS** 需要 Xcode 命令行工具和 `pkg-config`。

```sh
./scripts/build-macos-deps.sh
./scripts/package-macos.sh
```

**Windows** 需要 PowerShell 7.4 或更新版本。将 `-Architecture` 设为 `amd64` 或 `arm64`。

```powershell
pwsh -File scripts/package-windows.ps1 -Architecture amd64
```

## 发布新版本

1. 为要发布的提交推送稳定版本标签，例如 `v0.4.1`。[GitHub Actions](https://github.com/songkwon/relaydroid/actions/workflows/package.yml) 会构建四个安装包并创建 Release 草稿。
2. 在装有 Developer ID Application 证书和公证钥匙串配置、且已登录 GitHub CLI 的 Mac 上运行发布脚本。它会签名并公证两个 Mac 安装包，验证票据后发布草稿。

   ```sh
   ./scripts/publish-notarized-release.sh v0.4.1 TEAM_ID NOTARYTOOL_PROFILE
   ```

3. Release 公开后，流水线会验证 macOS 签名和公证票据，并用正式安装包的 SHA-256 更新 Homebrew cask。

本地构建时也可以通过 `RELAY_CODESIGN_IDENTITY` 选用签名证书，再通过 `RELAY_NOTARY_PROFILE` 使用已有的公证配置。

## 许可证

Relaydroid 使用 [MIT 许可证](LICENSE)。第三方许可随安装包提供，`libmtp` 和 `libusb` 的对应源码可在 Release 附件下载。详见[原生依赖说明](scripts/native-notices.txt)和 [Android 图标授权](assets/Android-icon-LICENSE.txt)。
