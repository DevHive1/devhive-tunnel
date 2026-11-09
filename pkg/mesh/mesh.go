// Package mesh bridges a real TUN device to the hive: encrypted IPv4 packets
// are relayed peer-to-peer through the hub, so members appear on one virtual
// LAN (broadcast/multicast replicated to every online member).
package mesh

import (
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"devhive-tunnel/pkg/client"
	"devhive-tunnel/pkg/tun"
)

// Mesh runs the IP relay.
type Mesh struct {
	c   *client.Client
	dev tun.Device
	mtu int

	selfFP string
	selfIP string
	subnet *net.IPNet

	mu   sync.RWMutex
	byIP map[string]string // vlan ip -> fingerprint
	byFP map[string]string // fingerprint -> vlan ip
	bcIP net.IP            // subnet broadcast

	inPkts, outPkts atomic.Uint64
	drops           atomic.Uint64
	done            chan struct{}
}

// Start begins relaying. It consumes client events until Stop/Close.
func Start(c *client.Client, dev tun.Device) (*Mesh, error) {
	info := c.Info()
	_, sn, err := net.ParseCIDR(info.Settings.SubnetCIDR)
	if err != nil {
		return nil, fmt.Errorf("subnet: %w", err)
	}
	m := &Mesh{
		c: c, dev: dev, mtu: 1400,
		selfFP: info.YourFP, selfIP: info.YourVLANIP, subnet: sn,
		byIP: map[string]string{}, byFP: map[string]string{},
		bcIP: broadcastOf(sn), done: make(chan struct{}),
	}
	for _, p := range c.Peers() {
		if p.VLANIP != "" && p.Online {
			m.byIP[p.VLANIP] = p.Fingerprint
			m.byFP[p.Fingerprint] = p.VLANIP
		}
	}
	go m.fromTUN()
	go m.fromHive()
	return m, nil
}

// Stats returns packet counters.
func (m *Mesh) Stats() (in, out, drops uint64) {
	return m.inPkts.Load(), m.outPkts.Load(), m.drops.Load()
}

// Close stops the relay (does not close the client or device).
func (m *Mesh) Close() { close(m.done) }

func broadcastOf(n *net.IPNet) net.IP {
	ip := n.IP.To4()
	if ip == nil {
		return net.IPv4bcast
	}
	b := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		b[i] = ip[i] | ^n.Mask[i]
	}
	return b
}

func (m *Mesh) isBroadcast(dst net.IP) bool {
	d4 := dst.To4()
	if d4 == nil {
		return false
	}
	return d4.Equal(net.IPv4bcast) || d4.Equal(m.bcIP) || d4.IsMulticast() || m.subnet.Contains(d4) && d4[3] == 255
}

func (m *Mesh) fromTUN() {
	buf := make([]byte, 65535)
	for {
		select {
		case <-m.done:
			return
		default:
		}
		n, err := m.dev.Read(buf)
		if err != nil {
			select {
			case <-m.done:
			default:
				log.Printf("[mesh] tun read: %v", err)
				time.Sleep(500 * time.Millisecond)
			}
			continue
		}
		if n < 20 || buf[0]>>4 != 4 {
			m.drops.Add(1)
			continue
		}
		dst := net.IP(buf[16:20])
		m.inPkts.Add(1)
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		if dst.Equal(net.ParseIP(m.selfIP)) {
			continue
		}
		if m.isBroadcast(dst) {
			peers := m.c.Peers()
			for _, p := range peers {
				if p.Online {
					if err := m.c.SendIP(p.Fingerprint, pkt); err != nil {
						m.drops.Add(1)
					}
				}
			}
			continue
		}
		m.mu.RLock()
		fp := m.byIP[dst.String()]
		m.mu.RUnlock()
		if fp == "" {
			if p := m.c.PeerByVLANIP(dst.String()); p != nil {
				fp = p.Fingerprint
			}
		}
		if fp == "" {
			m.drops.Add(1)
			continue
		}
		if err := m.c.SendIP(fp, pkt); err != nil {
			m.drops.Add(1)
		}
	}
}

func (m *Mesh) fromHive() {
	for {
		select {
		case <-m.done:
			return
		case ev, ok := <-m.c.Events:
			if !ok {
				return
			}
			switch ev.Type {
			case client.EvIP:
				if len(ev.Packet) == 0 {
					continue
				}
				if _, err := m.dev.Write(ev.Packet); err != nil {
					m.drops.Add(1)
				} else {
					m.outPkts.Add(1)
				}
			case client.EvPeerEvent:
				p := ev.Peer
				if p == nil {
					continue
				}
				m.mu.Lock()
				if p.Online {
					m.byIP[p.VLANIP] = p.Fingerprint
					m.byFP[p.Fingerprint] = p.VLANIP
				} else {
					delete(m.byIP, p.VLANIP)
					delete(m.byFP, p.Fingerprint)
				}
				m.mu.Unlock()
			case client.EvError:
				log.Printf("[mesh] %s", ev.Err)
			}
		}
	}
}
