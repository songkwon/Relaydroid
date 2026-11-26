package transfer

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
	"time"
)

type WiFiService struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	Pairing  bool   `json:"pairing"`
}

type PairResult struct {
	Device   *Device `json:"device,omitempty"`
	Endpoint string  `json:"endpoint,omitempty"`
}

func parseWiFiServices(out string) []WiFiService {
	services := []WiFiService{}
	seen := make(map[WiFiService]bool)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || checkEndpoint(fields[2]) != nil {
			continue
		}
		kind := strings.TrimSuffix(fields[1], ".")
		if kind != "_adb-tls-pairing._tcp" && kind != "_adb-tls-connect._tcp" {
			continue
		}
		s := WiFiService{Name: fields[0], Endpoint: fields[2], Pairing: kind == "_adb-tls-pairing._tcp"}
		if !seen[s] {
			services = append(services, s)
			seen[s] = true
		}
	}
	return services
}

func DiscoverWiFi(ctx context.Context) ([]WiFiService, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	out, err := runADB(ctx, "mdns", "services")
	if err != nil {
		return nil, err
	}
	return parseWiFiServices(string(out)), nil
}

func verifiedEndpoint(ctx context.Context, endpoint, name string) (Device, error) {
	for {
		devices, err := adbDevices(ctx)
		if err != nil {
			return Device{}, err
		}
		for _, d := range devices {
			if d.Kind != "adb-wifi" || d.State != "device" {
				continue
			}
			if d.ID == endpoint || (name != "" && strings.TrimSuffix(d.ID, ".") == name+"._adb-tls-connect._tcp") {
				return d, nil
			}
		}
		select {
		case <-ctx.Done():
			return Device{}, errors.New("等待手机授权或无线连接超时")
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func Pair(ctx context.Context, endpoint, code string) (PairResult, error) {
	if err := checkEndpoint(endpoint); err != nil {
		return PairResult{}, err
	}
	if !adbPairCodePattern.MatchString(code) {
		return PairResult{}, errors.New("请输入六位配对码")
	}
	pairCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	out, err := runADBInput(pairCtx, code+"\n", "pair", endpoint)
	cancel()
	if err != nil {
		return PairResult{}, err
	}
	message := strings.TrimSpace(string(out))
	if !strings.Contains(message, "Successfully paired") {
		return PairResult{}, errors.New(strings.ReplaceAll(strings.TrimPrefix(message, "Enter pairing code:"), code, "******"))
	}
	_, tail, hasGUID := strings.Cut(message, "[guid=")
	guid, _, _ := strings.Cut(tail, "]")
	if !hasGUID || !adbSerialPattern.MatchString(guid) {
		return PairResult{}, nil
	}

	// Pairing and connection use different services and ports. ADB can also
	// connect itself immediately. Match the GUID returned by this pairing,
	// since other phones can advertise over the same network.
	matchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(450 * time.Millisecond)
	defer ticker.Stop()
	for {
		devices, _ := adbDevices(matchCtx)
		for _, d := range devices {
			if d.Kind == "adb-wifi" && d.State == "device" && strings.TrimSuffix(d.ID, ".") == guid+"._adb-tls-connect._tcp" {
				return PairResult{Device: &d}, nil
			}
		}
		services, _ := DiscoverWiFi(matchCtx)
		for _, s := range services {
			if !s.Pairing && s.Name == guid {
				d, err := connectWiFi(matchCtx, s.Endpoint, guid)
				if err != nil {
					return PairResult{Endpoint: s.Endpoint}, nil
				}
				return PairResult{Device: &d}, nil
			}
		}
		select {
		case <-matchCtx.Done():
			return PairResult{}, nil
		case <-ticker.C:
		}
	}
}

func Connect(ctx context.Context, endpoint string) (Device, error) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	return connectWiFi(ctx, endpoint, "")
}

func connectWiFi(ctx context.Context, endpoint, service string) (Device, error) {
	if err := checkEndpoint(endpoint); err != nil {
		return Device{}, err
	}
	out, err := runADB(ctx, "connect", endpoint)
	if err != nil {
		return Device{}, err
	}
	message := strings.TrimSpace(string(out))
	if !strings.HasPrefix(message, "connected to ") && !strings.HasPrefix(message, "already connected to ") {
		if message == "" {
			message = "adb 未确认无线连接"
		}
		return Device{}, errors.New(message)
	}
	return verifiedEndpoint(ctx, endpoint, service)
}

type LegacySession struct {
	Endpoint  string
	USBSerial string
}

// Legacy Wi-Fi is only offered for a USB-authorised phone on the same local
// subnet. It deliberately uses a temporary port and restores USB mode at exit.
func StartLegacyWiFi(ctx context.Context, d Device) (LegacySession, Device, error) {
	if d.Kind != "adb-usb" || !adbSerialPattern.MatchString(d.ID) {
		return LegacySession{}, Device{}, errors.New("请先用 USB 连接手机并允许调试")
	}
	usb := adbRemote{d.ID, d.Transport}
	settingsCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	out, err := usb.shell(settingsCtx, "getprop ro.build.version.sdk; getprop ro.serialno; ip -o -4 addr show dev wlan0")
	cancel()
	if err != nil {
		return LegacySession{}, Device{}, err
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "\n", 3)
	if len(parts) != 3 {
		return LegacySession{}, Device{}, errors.New("请让手机连接同一个 Wi-Fi 后重试")
	}
	sdk, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || sdk >= 30 {
		return LegacySession{}, Device{}, errors.New("请在手机的「无线调试」中使用六位码配对")
	}
	serial := strings.TrimSpace(parts[1])
	if serial == "" || serial != d.ID {
		return LegacySession{}, Device{}, errors.New("无法验证 USB 手机的身份")
	}
	ip := parseWLAN(parts[2])
	if ip == nil || !ip.IsPrivate() || !onLocalSubnet(ip) {
		return LegacySession{}, Device{}, errors.New("请让手机和电脑连接同一可信 Wi-Fi，且两台设备能在局域网互通")
	}
	value, err := rand.Int(rand.Reader, big.NewInt(16000))
	if err != nil {
		return LegacySession{}, Device{}, err
	}
	port := strconv.Itoa(49152 + int(value.Int64()))
	s := LegacySession{Endpoint: net.JoinHostPort(ip.String(), port), USBSerial: serial}
	startCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	_, err = usb.run(startCtx, "tcpip", port)
	cancel()
	if err != nil {
		return rollbackLegacy(s, err)
	}

	connectCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	for {
		attempt, cancel := context.WithTimeout(connectCtx, 3*time.Second)
		wifi, err := connectWiFi(attempt, s.Endpoint, "")
		cancel()
		if err == nil {
			identity, cancel := context.WithTimeout(connectCtx, 3*time.Second)
			id, verify := (adbRemote{wifi.ID, wifi.Transport}).shell(identity, "getprop ro.serialno")
			cancel()
			if verify != nil || strings.TrimSpace(string(id)) != serial {
				return rollbackLegacy(s, errors.New("无线设备身份与 USB 手机不一致"))
			}
			return s, wifi, nil
		}
		select {
		case <-connectCtx.Done():
			return rollbackLegacy(s, fmt.Errorf("未能通过 Wi-Fi 连接手机：%w", err))
		case <-time.After(350 * time.Millisecond):
		}
	}
}

func parseWLAN(text string) net.IP {
	fields := strings.Fields(text)
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == "inet" {
			ip, _, err := net.ParseCIDR(fields[i+1])
			if err == nil {
				return ip.To4()
			}
		}
	}
	return nil
}

func onLocalSubnet(ip net.IP) bool {
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			_, network, err := net.ParseCIDR(addr.String())
			if err == nil && network.Contains(ip) {
				return true
			}
		}
	}
	return false
}

func rollbackLegacy(s LegacySession, cause error) (LegacySession, Device, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	if err := StopLegacyWiFi(ctx, s); err != nil {
		return s, Device{}, errors.Join(cause, err)
	}
	return LegacySession{}, Device{}, cause
}

func StopLegacyWiFi(ctx context.Context, s LegacySession) error {
	for _, target := range []struct {
		id       string
		wireless bool
	}{{s.Endpoint, true}, {s.USBSerial, false}} {
		if target.id == "" {
			continue
		}
		attempt, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
		if target.wireless {
			// Confirm identity before changing modes, even if the old Wi-Fi
			// address was reassigned while the device was away.
			out, err := runADB(attempt, "-s", target.id, "shell", "-T", "getprop ro.serialno")
			if err != nil || strings.TrimSpace(string(out)) != s.USBSerial {
				cancel()
				continue
			}
		}
		_, err := runADB(attempt, "-s", target.id, "usb")
		cancel()
		if err == nil {
			disconn, cancel := context.WithTimeout(ctx, time.Second)
			_, _ = runADB(disconn, "disconnect", s.Endpoint)
			cancel()
			return nil
		}
	}
	return errors.New("未能关闭手机上的临时无线调试，请插回 USB 重试，或在手机上关闭 USB 调试")
}
