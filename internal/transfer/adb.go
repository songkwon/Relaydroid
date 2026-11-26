package transfer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const adbRoot = "/sdcard"

var adbSerialPattern = regexp.MustCompile(`^[A-Za-z0-9._:\[\]%-]+$`)
var adbPairCodePattern = regexp.MustCompile(`^[0-9]{6}$`)

func adbExecutable() string {
	name := "adb"
	if filepath.Separator == '\\' {
		name += ".exe"
	}
	var candidates []string
	if bin, err := os.Executable(); err == nil {
		dir := filepath.Dir(bin)
		candidates = append(candidates, filepath.Join(dir, name), filepath.Join(dir, "..", "Resources", "platform-tools", name))
	}
	if bin, err := exec.LookPath(name); err == nil {
		candidates = append(candidates, bin)
	}
	for _, env := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if sdk := os.Getenv(env); sdk != "" {
			candidates = append(candidates, filepath.Join(sdk, "platform-tools", name))
		}
	}
	if filepath.Separator == '/' {
		candidates = append(candidates, "/opt/homebrew/bin/adb", "/usr/local/bin/adb", "/usr/bin/adb")
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, dir := range []string{"Library/Android/sdk", "Android/Sdk", "AppData/Local/Android/Sdk"} {
			candidates = append(candidates, filepath.Join(home, dir, "platform-tools", name))
		}
	}
	for _, bin := range candidates {
		if st, err := os.Stat(bin); err == nil && st.Mode().IsRegular() {
			return bin
		}
	}
	return ""
}

func runADB(ctx context.Context, args ...string) ([]byte, error) {
	return runADBInput(ctx, "", args...)
}

func runADBInput(ctx context.Context, input string, args ...string) ([]byte, error) {
	bin := adbExecutable()
	if bin == "" {
		return nil, errors.New("找不到 adb，请安装 Android SDK Platform-Tools")
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	configureCommand(cmd)
	cmd.WaitDelay = 2 * time.Second
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(string(out))
		}
		if detail != "" {
			return nil, errors.New(detail)
		}
		return nil, err
	}
	return out, nil
}

func adbDevices(ctx context.Context) ([]Device, error) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	out, err := runADB(ctx, "devices", "-l")
	if err != nil {
		return nil, err
	}
	return parseADBDevices(string(out)), nil
}

func parseADBDevices(out string) []Device {
	var devices []Device
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(line, "List of devices") {
			continue
		}
		if !adbSerialPattern.MatchString(fields[0]) {
			continue
		}
		d := Device{ID: fields[0], Kind: "adb-usb", Name: fields[0], State: fields[1], Root: adbRoot}
		if strings.HasPrefix(d.ID, "emulator-") {
			continue
		}
		if strings.Contains(d.ID, ":") || strings.HasSuffix(strings.TrimSuffix(d.ID, "."), "._adb-tls-connect._tcp") {
			d.Kind = "adb-wifi"
		}
		if len(fields) > 2 && fields[1] == "no" && fields[2] == "permissions" {
			d.State = "no-permissions"
		}
		for _, field := range fields[2:] {
			if model, ok := strings.CutPrefix(field, "model:"); ok {
				d.Name = strings.ReplaceAll(model, "_", " ")
			}
			if transport, ok := strings.CutPrefix(field, "transport_id:"); ok {
				d.Transport, _ = strconv.ParseUint(transport, 10, 64)
			}
		}
		devices = append(devices, d)
	}
	return devices
}

func checkEndpoint(endpoint string) error {
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil || net.ParseIP(host) == nil {
		return errors.New("请输入手机显示的 IP 地址与端口")
	}
	value, err := strconv.Atoi(port)
	if err != nil || value < 1 || value > 65535 {
		return errors.New("端口无效")
	}
	return nil
}

func Disconnect(ctx context.Context, serial string) error {
	devices, err := adbDevices(ctx)
	if err != nil {
		return err
	}
	for _, d := range devices {
		if d.ID == serial && d.Kind == "adb-wifi" {
			_, err := runADB(ctx, "disconnect", serial)
			return err
		}
	}
	return errors.New("Wi-Fi 设备已断开")
}

type adbRemote struct {
	id        string
	transport uint64
}

func (a adbRemote) close() {}

func (a adbRemote) run(ctx context.Context, args ...string) ([]byte, error) {
	target := []string{"-s", a.id}
	if a.transport != 0 {
		target = []string{"-t", strconv.FormatUint(a.transport, 10)}
	}
	return runADB(ctx, append(target, args...)...)
}

func quote(p string) string { return "'" + strings.ReplaceAll(p, "'", "'\"'\"'") + "'" }

func (a adbRemote) shell(ctx context.Context, command string) ([]byte, error) {
	return a.run(ctx, "shell", "-T", command)
}

func adbPath(p string) (string, error) {
	p = path.Clean(p)
	if strings.ContainsRune(p, 0) || (p != adbRoot && !strings.HasPrefix(p, adbRoot+"/")) {
		return "", errors.New("请选择 Android 共享存储中的路径")
	}
	return p, nil
}

func (a adbRemote) list(ctx context.Context, p string) ([]Entry, error) {
	p, err := adbPath(p)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	script := `for f do
		if [ -L "$f" ]; then kind=l
		elif [ -d "$f" ]; then kind=d
		elif [ -f "$f" ]; then kind=f
		else continue
		fi
		meta=$(stat -c '%s %Y' "$f") || continue
		printf '%s\000%s\000%s\000' "$kind" "$meta" "${f##*/}"
	done`
	out, err := a.shell(ctx, "find "+quote(p+"/.")+" -mindepth 1 -maxdepth 1 -exec sh -c "+quote(script)+" sh {} +")
	if err != nil {
		return nil, err
	}
	parts := bytes.Split(out, []byte{0})
	if (len(parts)-1)%3 != 0 || len(parts[len(parts)-1]) != 0 {
		return nil, errors.New("无法读取设备目录格式")
	}
	result := make([]Entry, 0, (len(parts)-1)/3)
	for i := 0; i+2 < len(parts); i += 3 {
		name := string(parts[i+2])
		if checkName(name) != nil {
			continue
		}
		meta := strings.Fields(string(parts[i+1]))
		if len(meta) != 2 {
			continue
		}
		sz, _ := strconv.ParseInt(meta[0], 10, 64)
		mod, _ := strconv.ParseInt(meta[1], 10, 64)
		kind := string(parts[i])
		result = append(result, Entry{name, path.Join(p, name), kind == "d", sz, mod, kind == "l"})
	}
	return result, nil
}

func (a adbRemote) pull(ctx context.Context, p, target string) error {
	src, err := adbPath(p)
	if err != nil {
		return err
	}
	_, err = a.run(ctx, "pull", "-a", src, target)
	return err
}

func (a adbRemote) push(ctx context.Context, source, parent, name string) (string, error) {
	parent, err := adbPath(parent)
	if err != nil {
		return "", err
	}
	target := path.Join(parent, name)
	out, err := a.shell(ctx, "mktemp -d "+quote(path.Join(parent, ".relay.XXXXXXXX")))
	if err != nil {
		return "", err
	}
	stage, err := adbPath(strings.TrimSpace(string(out)))
	if err != nil || path.Dir(stage) != parent || !strings.HasPrefix(path.Base(stage), ".relay.") {
		return "", errors.New("Android 返回了无效的临时目录")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = a.shell(cleanup, "rm -rf -- "+quote(stage))
	}()
	temp := path.Join(stage, "data")
	if _, err = a.run(ctx, "push", source, temp); err != nil {
		return "", err
	}
	return target, a.move(ctx, temp, target)
}

func (a adbRemote) mkdir(ctx context.Context, parent, name string) (string, error) {
	p, err := adbPath(path.Join(parent, name))
	if err != nil {
		return "", err
	}
	_, err = a.shell(ctx, "mkdir -- "+quote(p))
	return p, err
}

func (a adbRemote) move(ctx context.Context, src, target string) error {
	// Android 10's Toybox does not implement mv -T. Check for an
	// existing target, retain -n, and verify that the source was moved.
	command := fmt.Sprintf(`src=%s; target=%s
		if [ -e "$target" ] || [ -L "$target" ]; then
			printf '目标已有同名项目\n' >&2
			exit 17
		fi
		if ! mv -n -- "$src" "$target"; then exit 1; fi
		if [ -e "$src" ] || [ -L "$src" ]; then
			printf '目标已有同名项目\n' >&2
			exit 17
		fi`, quote(src), quote(target))
	_, err := a.shell(ctx, command)
	if err != nil && ctx.Err() == nil {
		return fmt.Errorf("无法移动到 %s：%w", path.Base(target), err)
	}
	return err
}

func (a adbRemote) rename(ctx context.Context, src, name string) error {
	src, err := adbPath(src)
	if err != nil || src == adbRoot {
		return errors.New("无法重命名共享存储根目录")
	}
	return a.move(ctx, src, path.Join(path.Dir(src), name))
}

func (a adbRemote) remove(ctx context.Context, src string) error {
	src, err := adbPath(src)
	if err != nil || src == adbRoot {
		return errors.New("无法删除共享存储根目录")
	}
	_, err = a.shell(ctx, "rm -r -- "+quote(src))
	return err
}
