package mesh

import (
	"net"
	"testing"
)

func TestBroadcastOf(t *testing.T) {
	_, n, _ := net.ParseCIDR("100.78.0.0/16")
	got := broadcastOf(n).String()
	if got != "100.78.255.255" {
		t.Fatalf("broadcast = %s", got)
	}
}

func TestIsBroadcast(t *testing.T) {
	_, n, _ := net.ParseCIDR("100.78.0.0/16")
	m := &Mesh{subnet: n, bcIP: broadcastOf(n)}
	yes := []string{"255.255.255.255", "100.78.255.255", "224.0.0.251", "239.255.255.250"}
	for _, s := range yes {
		if !m.isBroadcast(net.ParseIP(s)) {
			t.Errorf("%s should be broadcast", s)
		}
	}
	no := []string{"100.78.0.1", "100.78.12.7", "8.8.8.8"}
	for _, s := range no {
		if m.isBroadcast(net.ParseIP(s)) {
			t.Errorf("%s should not be broadcast", s)
		}
	}
}
