//go:build mtp && cgo && (darwin || linux)

package transfer

/*
#cgo pkg-config: libmtp libusb-1.0
#include <libmtp.h>
#include <libusb.h>
#include <stdint.h>
#include <stdlib.h>

extern int relayMTPCancel(uintptr_t key);

// Work only from the USB configuration already enumerated by the OS.
// Opening devices or probing Microsoft OS descriptors during refresh can
// contend with ADB, including when the phone exposes only its ADB interface.
static int relay_mtp_interface(libusb_device *usb, int known_mtp) {
	struct libusb_config_descriptor *config = NULL;
	if (libusb_get_active_config_descriptor(usb, &config) != 0) return 0;
	int found = 0;
	for (int i = 0; !found && i < config->bNumInterfaces; i++) {
		const struct libusb_interface *iface = &config->interface[i];
		for (int j = 0; !found && j < iface->num_altsetting; j++) {
			const struct libusb_interface_descriptor *alt = &iface->altsetting[j];
			int standard = alt->bInterfaceClass == LIBUSB_CLASS_IMAGE;
			int vendor = known_mtp && alt->bInterfaceClass == LIBUSB_CLASS_VENDOR_SPEC &&
				alt->bInterfaceSubClass == 0 && alt->bInterfaceProtocol == 0;
			if ((!standard && !vendor) || alt->bNumEndpoints != 3) continue;
			int in = 0, out = 0, intr = 0;
			for (int k = 0; k < alt->bNumEndpoints; k++) {
				const struct libusb_endpoint_descriptor *ep = &alt->endpoint[k];
				int type = ep->bmAttributes & LIBUSB_TRANSFER_TYPE_MASK;
				int incoming = (ep->bEndpointAddress & LIBUSB_ENDPOINT_DIR_MASK) == LIBUSB_ENDPOINT_IN;
				if (type == LIBUSB_TRANSFER_TYPE_BULK && incoming) in = 1;
				if (type == LIBUSB_TRANSFER_TYPE_BULK && !incoming) out = 1;
				if (type == LIBUSB_TRANSFER_TYPE_INTERRUPT && incoming) intr = 1;
			}
			found = in && out && intr;
		}
	}
	libusb_free_config_descriptor(config);
	return found;
}

static int relay_mtp_scan(libusb_context *ctx, LIBMTP_raw_device_t **out, int *count) {
	libusb_device **devices;
	ssize_t n = libusb_get_device_list(ctx, &devices);
	if (n < 0) return (int)n;
	LIBMTP_raw_device_t *result = calloc((size_t)n, sizeof(*result));
	if (n > 0 && !result) {
		libusb_free_device_list(devices, 1);
		return LIBUSB_ERROR_NO_MEM;
	}
	LIBMTP_device_entry_t *table = NULL;
	int entries = 0;
	LIBMTP_Get_Supported_Devices_List(&table, &entries);
	int used = 0;
	for (ssize_t i = 0; i < n; i++) {
		libusb_device *usb = devices[i];
		struct libusb_device_descriptor descriptor;
		if (libusb_get_device_descriptor(usb, &descriptor) != 0) continue;
		LIBMTP_device_entry_t *known = NULL;
		for (int j = 0; j < entries; j++) {
			if (table[j].vendor_id == descriptor.idVendor && table[j].product_id == descriptor.idProduct) {
				known = &table[j];
				break;
			}
		}
		if (!relay_mtp_interface(usb, known != NULL)) continue;
		LIBMTP_raw_device_t *device = &result[used++];
		if (known) device->device_entry = *known;
		device->device_entry.vendor_id = descriptor.idVendor;
		device->device_entry.product_id = descriptor.idProduct;
		device->bus_location = libusb_get_bus_number(usb);
		device->devnum = libusb_get_device_address(usb);
	}
	libusb_free_device_list(devices, 1);
	*out = result;
	*count = used;
	return 0;
}


static int relay_mtp_progress(uint64_t sent, uint64_t total, const void *data) {
	(void)sent;
	(void)total;
	return relayMTPCancel((uintptr_t)data);
}

static int relay_mtp_get(LIBMTP_mtpdevice_t *d, uint32_t id, const char *target, uintptr_t key) {
	return LIBMTP_Get_File_To_File(d, id, target, relay_mtp_progress, (const void *)key);
}

static int relay_mtp_send(LIBMTP_mtpdevice_t *d, const char *source, LIBMTP_file_t *file, uintptr_t key) {
	return LIBMTP_Send_File_From_File(d, source, file, relay_mtp_progress, (const void *)key);
}
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"runtime/cgo"
	"strconv"
	"strings"
	"sync"
	"unicode/utf16"
	"unsafe"
)

var mtpGate = make(chan struct{}, 1)
var initMTP sync.Once

func lockMTP(ctx context.Context) error {
	select {
	case mtpGate <- struct{}{}:
		initMTP.Do(func() { C.LIBMTP_Init() })
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func unlockMTP() { <-mtpGate }

var discovery struct {
	once    sync.Once
	context *C.libusb_context
	err     error
}

func rawMTP() ([]C.LIBMTP_raw_device_t, func(), error) {
	discovery.once.Do(func() {
		var usb *C.libusb_context
		if code := C.libusb_init(&usb); code != 0 {
			discovery.err = fmt.Errorf("无法初始化 USB 设备扫描（%d）", int(code))
		}
		discovery.context = usb
	})
	if discovery.err != nil {
		return nil, func() {}, discovery.err
	}
	var raw *C.LIBMTP_raw_device_t
	var count C.int
	code := C.relay_mtp_scan(discovery.context, &raw, &count)
	release := func() { C.free(unsafe.Pointer(raw)) }
	if code != 0 {
		return nil, release, fmt.Errorf("无法读取 USB 设备描述符（%d）", int(code))
	}
	return unsafe.Slice(raw, int(count)), release, nil
}

func mtpID(d C.LIBMTP_raw_device_t) string {
	return fmt.Sprintf("%d.%d:%04x:%04x", uint32(d.bus_location), uint8(d.devnum), uint16(d.device_entry.vendor_id), uint16(d.device_entry.product_id))
}

func mtpDevices(ctx context.Context) ([]Device, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, release, err := rawMTP()
	defer release()
	if err != nil {
		return nil, err
	}
	devices := make([]Device, 0, len(raw))
	for _, d := range raw {
		name := "USB 文件传输设备"
		if d.device_entry.vendor != nil {
			name = C.GoString(d.device_entry.vendor) + " USB 存储"
		}
		devices = append(devices, Device{ID: mtpID(d), Kind: "mtp", Name: name, State: "device", Root: "/"})
	}
	return devices, nil
}

type mtpRemote struct {
	device *C.LIBMTP_mtpdevice_t
	names  map[string]map[string]bool
}

func openMTP(ctx context.Context, id string) (remote, error) {
	if err := lockMTP(ctx); err != nil {
		return nil, err
	}
	raw, release, err := rawMTP()
	defer release()
	if err != nil {
		unlockMTP()
		return nil, err
	}
	for i := range raw {
		if mtpID(raw[i]) != id {
			continue
		}
		device := C.LIBMTP_Open_Raw_Device_Uncached(&raw[i])
		if device == nil {
			unlockMTP()
			return nil, errors.New("无法打开手机，请解锁屏幕并确认 USB 用途为「文件传输」，同时退出其他 Android 文件管理工具")
		}
		C.LIBMTP_Clear_Errorstack(device)
		return &mtpRemote{device: device, names: make(map[string]map[string]bool)}, nil
	}
	unlockMTP()
	return nil, errors.New("USB 设备已断开")
}

func (m *mtpRemote) close() {
	C.LIBMTP_Release_Device(m.device)
	unlockMTP()
}

func (m *mtpRemote) failure(label string) error {
	var details []string
	for e := C.LIBMTP_Get_Errorstack(m.device); e != nil; e = e.next {
		if e.error_text != nil {
			details = append(details, C.GoString(e.error_text))
		}
	}
	C.LIBMTP_Clear_Errorstack(m.device)
	if len(details) > 0 {
		return fmt.Errorf("%s：%s", label, strings.Join(details, " / "))
	}
	return errors.New(label)
}

func mtpPath(p string) (storage, object C.uint32_t, err error) {
	if p == "/" || !strings.HasPrefix(p, "/") || path.Clean(p) != p {
		return 0, 0, errors.New("请选择设备中的存储空间或文件夹")
	}
	segments := strings.Split(p[1:], "/")
	for i, segment := range segments {
		id, err := strconv.ParseUint(segment, 16, 32)
		if err != nil || id == 0 || id == 0xffffffff || len(segment) != 8 {
			return 0, 0, errors.New("MTP 文件路径无效")
		}
		if i == 0 {
			storage = C.uint32_t(id)
		} else {
			object = C.uint32_t(id)
		}
	}
	return storage, object, nil
}

func mtpJoin(parent string, id C.uint32_t) string {
	return path.Join(parent, fmt.Sprintf("%08x", uint32(id)))
}

func (m *mtpRemote) list(ctx context.Context, p string) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	items := []Entry{}
	C.LIBMTP_Clear_Errorstack(m.device)
	if p == "/" {
		if C.LIBMTP_Get_Storage(m.device, C.LIBMTP_STORAGE_SORTBY_NOTSORTED) != 0 {
			return nil, m.failure("无法读取设备存储空间")
		}
		for s := m.device.storage; s != nil; s = s.next {
			name := C.GoString(s.StorageDescription)
			if name == "" {
				name = "内部存储"
			}
			items = append(items, Entry{Name: name, Path: mtpJoin("/", s.id), Directory: true})
		}
		if len(items) == 0 {
			return nil, errors.New("请解锁手机，并允许通过 USB 访问手机数据")
		}
		return items, nil
	}
	storage, parent, err := mtpPath(p)
	if err != nil {
		return nil, err
	}
	if parent == 0 {
		parent = C.LIBMTP_FILES_AND_FOLDERS_ROOT
	}
	entries := C.LIBMTP_Get_Files_And_Folders(m.device, storage, parent)
	for f := entries; f != nil; {
		next := f.next
		name := C.GoString(f.filename)
		if checkName(name) == nil {
			items = append(items, Entry{Name: name, Path: mtpJoin(p, f.item_id), Directory: f.filetype == C.LIBMTP_FILETYPE_FOLDER, Size: int64(f.filesize), Modified: int64(f.modificationdate)})
		}
		f.next = nil
		C.LIBMTP_destroy_file_t(f)
		f = next
	}
	if C.LIBMTP_Get_Errorstack(m.device) != nil {
		return nil, m.failure("无法完整读取设备文件夹")
	}
	return items, nil
}

func (m *mtpRemote) available(ctx context.Context, p, name string) error {
	if len(utf16.Encode([]rune(name))) > 254 {
		return errors.New("文件名超出 MTP 的长度限制")
	}
	if _, ok := m.names[p]; !ok {
		items, err := m.list(ctx, p)
		if err != nil {
			return err
		}
		m.names[p] = make(map[string]bool, len(items))
		for _, item := range items {
			m.names[p][item.Name] = true
		}
	}
	if m.names[p][name] {
		return errors.New("目标已有同名项目")
	}
	return ctx.Err()
}

//export relayMTPCancel
func relayMTPCancel(key C.uintptr_t) C.int {
	if cgo.Handle(key).Value().(context.Context).Err() != nil {
		return 1
	}
	return 0
}

func (m *mtpRemote) pull(ctx context.Context, p, target string) error {
	_, id, err := mtpPath(p)
	if err != nil {
		return err
	}
	if id == 0 {
		return errors.New("无法把存储空间当作文件下载")
	}
	filename := C.CString(target)
	defer C.free(unsafe.Pointer(filename))
	key := cgo.NewHandle(ctx)
	defer key.Delete()
	C.LIBMTP_Clear_Errorstack(m.device)
	if C.relay_mtp_get(m.device, id, filename, C.uintptr_t(key)) != 0 {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return m.failure("从手机读取文件失败")
	}
	return nil
}

func (m *mtpRemote) push(ctx context.Context, source, p, name string) (string, error) {
	storage, parent, err := mtpPath(p)
	if err != nil {
		return "", err
	}
	if err := m.available(ctx, p, name); err != nil {
		return "", err
	}
	info, err := os.Stat(source)
	if err != nil {
		return "", err
	}
	f := C.LIBMTP_new_file_t()
	if f == nil {
		return "", errors.New("无法为 MTP 分配文件信息")
	}
	defer C.LIBMTP_destroy_file_t(f)
	f.filename = C.CString(name)
	f.filesize = C.uint64_t(info.Size())
	f.modificationdate = C.time_t(info.ModTime().Unix())
	f.storage_id, f.parent_id = storage, parent
	if parent == 0 {
		f.parent_id = C.LIBMTP_FILES_AND_FOLDERS_ROOT
	}
	f.filetype = C.LIBMTP_FILETYPE_UNKNOWN
	switch strings.ToLower(path.Ext(name)) {
	case ".jpg", ".jpeg":
		f.filetype = C.LIBMTP_FILETYPE_JPEG
	case ".png":
		f.filetype = C.LIBMTP_FILETYPE_PNG
	case ".gif":
		f.filetype = C.LIBMTP_FILETYPE_GIF
	case ".mp3":
		f.filetype = C.LIBMTP_FILETYPE_MP3
	case ".mp4":
		f.filetype = C.LIBMTP_FILETYPE_MP4
	case ".wav":
		f.filetype = C.LIBMTP_FILETYPE_WAV
	}
	filename := C.CString(source)
	defer C.free(unsafe.Pointer(filename))
	key := cgo.NewHandle(ctx)
	defer key.Delete()
	C.LIBMTP_Clear_Errorstack(m.device)
	if C.relay_mtp_send(m.device, filename, f, C.uintptr_t(key)) != 0 {
		err := m.failure("写入手机失败")
		if f.item_id != 0 && C.LIBMTP_Delete_Object(m.device, f.item_id) != 0 {
			err = errors.Join(err, m.failure("清理未完成的设备文件失败"))
		}
		if ctx.Err() != nil {
			return "", errors.Join(ctx.Err(), err)
		}
		return "", err
	}
	m.names[p][name] = true
	return mtpJoin(p, f.item_id), nil
}

func (m *mtpRemote) mkdir(ctx context.Context, p, name string) (string, error) {
	storage, parent, err := mtpPath(p)
	if err != nil {
		return "", err
	}
	if err := m.available(ctx, p, name); err != nil {
		return "", err
	}
	filename := C.CString(name)
	defer C.free(unsafe.Pointer(filename))
	if parent == 0 {
		parent = C.LIBMTP_FILES_AND_FOLDERS_ROOT
	}
	C.LIBMTP_Clear_Errorstack(m.device)
	id := C.LIBMTP_Create_Folder(m.device, filename, parent, storage)
	if id == 0 {
		return "", m.failure("无法在设备上新建文件夹")
	}
	m.names[p][name] = true
	m.names[mtpJoin(p, id)] = make(map[string]bool)
	return mtpJoin(p, id), nil
}

func (m *mtpRemote) rename(ctx context.Context, p, name string) error {
	_, id, err := mtpPath(p)
	if err != nil || id == 0 {
		return errors.New("无法重命名设备存储空间")
	}
	if err := m.available(ctx, path.Dir(p), name); err != nil {
		return err
	}
	C.LIBMTP_Clear_Errorstack(m.device)
	f := C.LIBMTP_Get_Filemetadata(m.device, id)
	if f == nil {
		return m.failure("找不到要重命名的项目")
	}
	defer C.LIBMTP_destroy_file_t(f)
	filename := C.CString(name)
	defer C.free(unsafe.Pointer(filename))
	if C.LIBMTP_Set_File_Name(m.device, f, filename) != 0 {
		return m.failure("设备无法重命名这个项目")
	}
	return nil
}

func (m *mtpRemote) remove(ctx context.Context, p string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, id, err := mtpPath(p)
	if err != nil || id == 0 {
		return errors.New("无法删除设备存储空间")
	}
	C.LIBMTP_Clear_Errorstack(m.device)
	if C.LIBMTP_Delete_Object(m.device, id) != 0 {
		return m.failure("设备无法删除这个项目")
	}
	delete(m.names, path.Dir(p))
	delete(m.names, p)
	return nil
}
