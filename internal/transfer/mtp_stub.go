//go:build !mtp || !cgo || (!darwin && !linux)

package transfer

import (
	"context"
	"errors"
)

var errMTPUnavailable = errors.New("此构建支持 ADB。USB 文件传输（MTP）需要启用 libmtp 的 macOS 或 Linux 构建")

func mtpDevices(context.Context) ([]Device, error)    { return nil, errMTPUnavailable }
func openMTP(context.Context, string) (remote, error) { return nil, errMTPUnavailable }

func ReleaseMTP(context.Context) {}
