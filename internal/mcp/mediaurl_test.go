package mcp

import (
	"fmt"
	"net"
	"testing"
)

// fakeDNS stands in for the resolver: the tests run without one.
func fakeDNS(t *testing.T) {
	answers := map[string]string{
		"example.com": "93.184.215.14", "cdn.example.com": "93.184.215.15",
		"169.254.169.254.nip.io": "169.254.169.254", "localtest.me": "127.0.0.1", "172-18-0-5.nip.io": "172.18.0.5",
	}
	previous := lookupHost
	lookupHost = func(host string) ([]net.IP, error) {
		if a, ok := answers[host]; ok {
			return []net.IP{net.ParseIP(a)}, nil
		}
		return nil, fmt.Errorf("no such host %s", host)
	}
	t.Cleanup(func() { lookupHost = previous })
}

// A media URL is handed to Evolution, which fetches it from inside the Docker
// network. Anything that resolves back into that network turns "send this
// image" into a read of an internal service delivered to a phone number.
func TestMediaURLRefusesTheServersOwnNetwork(t *testing.T) {
	fakeDNS(t)
	refused := []string{
		"http://169.254.169.254.nip.io/latest/meta-data/",
		"http://localtest.me:15672/api/overview",
		"http://172-18-0-5.nip.io:15672/",
		"http://127.1/",
		"http://127.0.0.1./",
		"http://127.0.0.1:15672/api/overview",
		"http://localhost/",
		"http://rabbitmq:15672/api/queues",
		"http://minio:9000/evolution-media/",
		"http://postgres-mcp:5432/",
		"http://10.0.0.5/secret",
		"http://192.168.1.10/secret",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]/",
		"file:///etc/passwd",
		"gopher://evolution-go:4000/",
		"http://user:pass@example.com/photo.jpg",
		"http://evolution-go/",
		"http://intranet/photo.jpg",
	}
	for _, raw := range refused {
		if err := checkMediaURL(raw); err == nil {
			t.Errorf("checkMediaURL(%q) allowed it", raw)
		}
	}

	allowed := []string{
		"https://example.com/photo.jpg",
		"http://example.com/photo.jpg",
		"https://cdn.example.com:8443/a/b/c.pdf?token=1",
		"https://203.0.113.7/photo.jpg",
	}
	for _, raw := range allowed {
		if err := checkMediaURL(raw); err != nil {
			t.Errorf("checkMediaURL(%q) refused it: %v", raw, err)
		}
	}
}
