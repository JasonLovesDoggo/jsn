//go:build !darwin || !cgo

package main

import (
	"context"
	"fmt"
)

func capture(context.Context, *recorder, bool) error {
	return fmt.Errorf("mousetrace requires macOS and a build with CGO_ENABLED=1")
}

func requestAccess() error {
	return fmt.Errorf("Input Monitoring access requests require macOS and CGO_ENABLED=1")
}
