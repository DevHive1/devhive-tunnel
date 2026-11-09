//go:build !linux && !windows

package tun

import (
	"errors"
	"runtime"
)

// Device is unavailable on this platform.
type stubDevice struct{}

func Open(name string) (Device, error) {
	return stubDevice{}, errors.New("real TUN is unavailable on " + runtime.GOOS +
		" — Termux/Android cannot create a TUN device without root. Use chat/files, or run LAN mode from a desktop")
}

func (stubDevice) Name() string              { return "" }
func (stubDevice) Read([]byte) (int, error)  { return 0, errors.New("tun unavailable") }
func (stubDevice) Write([]byte) (int, error) { return 0, errors.New("tun unavailable") }
func (stubDevice) Close() error              { return nil }
func Configure(Device, string, string) error { return errors.New("tun unavailable") }
