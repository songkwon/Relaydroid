//go:build !darwin

package main

import (
	"context"
	"errors"

	"github.com/songkwon/relaydroid/internal/transfer"
)

type finderMount struct{}

func (r *Relaydroid) FinderMount(context.Context, transfer.Device) (string, error) {
	return "", errors.New("Finder 挂载仅在 macOS 可用")
}
func (r *Relaydroid) FinderUnmount() error { return nil }
