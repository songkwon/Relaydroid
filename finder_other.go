//go:build !darwin

package main

import (
	"context"
	"errors"

	"github.com/songkwon/Relay/internal/transfer"
)

type finderMount struct{}

func (r *Relay) FinderMount(context.Context, transfer.Device) (string, error) {
	return "", errors.New("Finder 挂载仅在 macOS 可用")
}
func (r *Relay) FinderUnmount() error { return nil }
