//go:build windows

package tun

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	procCreateEvent   = kernel32.NewProc("CreateEventW")
	procWaitForSingle = kernel32.NewProc("WaitForSingleObject")

	wintun             = syscall.NewLazyDLL("wintun.dll")
	procOpenAdapter    = wintun.NewProc("WintunOpenAdapter")
	procCreateAdapter  = wintun.NewProc("WintunCreateAdapter")
	procCloseAdapter   = wintun.NewProc("WintunCloseAdapter")
	procStartSession   = wintun.NewProc("WintunStartSession")
	procEndSession     = wintun.NewProc("WintunEndSession")
	procReceivePacket  = wintun.NewProc("WintunReceivePacket")
	procReleasePacket  = wintun.NewProc("WintunReleaseReceivePacket")
	procAllocatePacket = wintun.NewProc("WintunAllocateSendPacket")
	procSendPacket     = wintun.NewProc("WintunSendPacket")
	procReadWaitEvent  = wintun.NewProc("WintunGetReadWaitEvent")
)

const (
	wintunRingCapacity = 0x400000 // 4 MiB
	infinite           = 0xFFFFFFFF
	errorNoMoreItems   = 259
)

// Device is a Windows Wintun adapter. Place wintun.dll next to devtun.exe
// (or in PATH); download it from https://www.wintun.net/.
type winDevice struct {
	adapter   uintptr
	session   uintptr
	waitEvent syscall.Handle
	name      string
}

// Open creates or opens the named Wintun adapter (needs Administrator).
func Open(name string) (Device, error) {
	if name == "" || name == "devhive0" {
		name = "devhive-tunnel"
	}
	n, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	typ, _ := syscall.UTF16PtrFromString("devhive")
	adapter, _, _ := procOpenAdapter.Call(uintptr(unsafe.Pointer(n)))
	if adapter == 0 {
		adapter, _, _ = procCreateAdapter.Call(uintptr(unsafe.Pointer(n)), uintptr(unsafe.Pointer(typ)), 0)
	}
	if adapter == 0 {
		return nil, errors.New("wintun: cannot create adapter — is wintun.dll present and are you Administrator?")
	}
	session, _, _ := procStartSession.Call(adapter, wintunRingCapacity)
	if session == 0 {
		procCloseAdapter.Call(adapter)
		return nil, errors.New("wintun: cannot start session")
	}
	ev, _, _ := procReadWaitEvent.Call(session)
	evHandle, _, _ := procCreateEvent.Call(0, 0, 0, 0)
	d := &winDevice{adapter: adapter, session: session, name: name}
	d.waitEvent = syscall.Handle(evHandle)
	_ = ev
	return d, nil
}

func (d *winDevice) Name() string { return d.name }

// Read blocks until one IP packet is available.
func (d *winDevice) Read(p []byte) (int, error) {
	for {
		var size uintptr
		pkt, _, _ := procReceivePacket.Call(d.session, uintptr(unsafe.Pointer(&size)))
		if pkt != 0 {
			n := copy(p, unsafe.Slice((*byte)(unsafe.Pointer(pkt)), int(size)))
			procReleasePacket.Call(d.session, pkt)
			return n, nil
		}
		// No packet queued: wait for the session read event.
		ev, _, _ := procReadWaitEvent.Call(d.session)
		if ev == 0 {
			return 0, errors.New("wintun: session closed")
		}
		procWaitForSingle.Call(ev, infinite)
	}
}

// Write queues one IP packet.
func (d *winDevice) Write(p []byte) (int, error) {
	pkt, _, _ := procAllocatePacket.Call(d.session, uintptr(len(p)))
	if pkt == 0 {
		return 0, errors.New("wintun: ring full")
	}
	copy(unsafe.Slice((*byte)(unsafe.Pointer(pkt)), len(p)), p)
	procSendPacket.Call(d.session, pkt)
	return len(p), nil
}

func (d *winDevice) Close() error {
	if d.session != 0 {
		procEndSession.Call(d.session)
		d.session = 0
	}
	if d.adapter != 0 {
		procCloseAdapter.Call(d.adapter)
		d.adapter = 0
	}
	if d.waitEvent != 0 {
		syscall.CloseHandle(d.waitEvent)
	}
	return nil
}

// Configure assigns the hive IP, route and MTU through netsh.
func Configure(d Device, addrCIDR, subnetCIDR string) error {
	ip := addrCIDR
	if i := strings.IndexByte(ip, '/'); i >= 0 {
		ip = ip[:i]
	}
	name := d.Name()
	steps := [][]string{
		{"interface", "ipv4", "set", "address", "name=" + name, "static", ip, "255.255.0.0"},
		{"interface", "ipv4", "set", "subinterface", name, "mtu=1400", "store=active"},
		{"interface", "ipv4", "add", "route", "prefix=" + subnetCIDR, "interface=" + name, "store=active"},
	}
	var lastErr error
	for _, args := range steps {
		if out, err := exec.Command("netsh", args...).CombinedOutput(); err != nil {
			lastErr = fmt.Errorf("netsh %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
		}
	}
	return lastErr
}
