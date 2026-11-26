package transfer

import "testing"

func TestADBDeviceTransport(t *testing.T) {
	devices := parseADBDevices(`List of devices attached
sampler-123  device usb:1 product:sample model:Demo_Phone transport_id:12
192.168.2.7:42613 device product:sample model:Demo_Phone transport_id:14
adb-other._adb-tls-connect._tcp. offline transport_id:15
emulator-5554 device product:sdk model:Example transport_id:16
`)
	if len(devices) != 3 {
		t.Fatalf("got %d phones, want 3", len(devices))
	}
	if usb := devices[0]; usb.Kind != "adb-usb" || usb.Transport != 12 || usb.Name != "Demo Phone" {
		t.Fatalf("USB selection lost the transport: %+v", usb)
	}
	if wifi := devices[1]; wifi.Kind != "adb-wifi" || wifi.Transport != 14 || wifi.State != "device" {
		t.Fatalf("Wi-Fi selection lost the transport: %+v", wifi)
	}
	if offline := devices[2]; offline.Kind != "adb-wifi" || offline.State != "offline" {
		t.Fatalf("mDNS device was classified incorrectly: %+v", offline)
	}
}

func TestDiscoverOnlyTLSWiFiServices(t *testing.T) {
	got := parseWiFiServices(`List of discovered mdns services
adb-phone    _adb-tls-pairing._tcp.    192.168.2.7:40110
adb-phone    _adb-tls-pairing._tcp.    192.168.2.7:40110
adb-phone    _adb-tls-connect._tcp     192.168.2.7:42500
adb-legacy   _adb._tcp                 192.168.2.8:5555
invalid      _adb-tls-connect._tcp    example.org:443
`)
	if len(got) != 2 || !got[0].Pairing || got[1].Pairing || got[0].Endpoint == got[1].Endpoint {
		t.Fatalf("expected distinct pairing and connection services: %+v", got)
	}
}

func TestLegacyWLANAddress(t *testing.T) {
	addr := parseWLAN("25: wlan0    inet 192.168.2.7/24 brd 192.168.2.255 scope global wlan0")
	if addr == nil || addr.String() != "192.168.2.7" {
		t.Fatalf("unexpected WLAN address: %v", addr)
	}
	if parseWLAN("wlan0") != nil {
		t.Fatal("found an address without an IPv4 binding")
	}
}
