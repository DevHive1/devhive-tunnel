// Package tun opens a real layer-3 tunnel device on desktops so hive peers
// share a virtual LAN (100.x/16) for LAN games and direct IP apps.
package tun

import "io"

// Device is an OS tunnel interface carrying raw IP packets.
type Device interface {
	io.ReadWriteCloser
	Name() string
}
