//go:build linux

package tun

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

const (
	tunSetIff = 0x400454ca
	iffTun    = 0x0001
	iffNoPI   = 0x1000
	ifNameSz  = 16
)

// Device is a Linux TUN interface.
type linuxDevice struct {
	f    *os.File
	name string
}

// Open creates (or attaches) a TUN device. Needs root or CAP_NET_ADMIN.
func Open(name string) (Device, error) {
	if name == "" {
		name = "devhive0"
	}
	if len(name) >= ifNameSz {
		return nil, fmt.Errorf("interface name too long")
	}
	f, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open /dev/net/tun: %w (needs root or CAP_NET_ADMIN)", err)
	}
	var ifr [40]byte
	copy(ifr[:ifNameSz-1], name)
	*(*uint16)(unsafe.Pointer(&ifr[ifNameSz])) = iffTun | iffNoPI
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), tunSetIff, uintptr(unsafe.Pointer(&ifr[0])))
	if errno != 0 {
		f.Close()
		return nil, fmt.Errorf("TUNSETIFF: %w", errno)
	}
	got := string(ifr[:ifNameSz])
	if i := strings.IndexByte(got, 0); i >= 0 {
		got = got[:i]
	}
	return &linuxDevice{f: f, name: got}, nil
}

func (d *linuxDevice) Name() string                { return d.name }
func (d *linuxDevice) Read(p []byte) (int, error)  { return d.f.Read(p) }
func (d *linuxDevice) Write(p []byte) (int, error) { return d.f.Write(p) }
func (d *linuxDevice) Close() error                { return d.f.Close() }

// Configure assigns the hive IP and brings the interface up.
func Configure(d Device, addrCIDR, subnetCIDR string) error {
	name := d.Name()
	steps := [][]string{
		{"link", "set", "dev", name, "mtu", "1400", "up"},
		{"addr", "replace", addrCIDR, "dev", name},
		{"route", "replace", subnetCIDR, "dev", name},
	}
	for _, args := range steps {
		cmd := exec.Command("ip", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("ip %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}
