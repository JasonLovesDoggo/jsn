//go:build !darwin || !cgo

package main

import (
	"context"
	"errors"
)

var errUnsupported = errors.New("mousetrace requires macOS and a build with CGO_ENABLED=1")

func capture(context.Context, *recorder, options) error { return errUnsupported }

func requestAccess() error { return errUnsupported }

func accessibilityTrusted(bool) bool { return false }
