//go:build !windows

package transfer

import "os/exec"

func configureCommand(_ *exec.Cmd) {}
