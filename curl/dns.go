package curl

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// dnsServers are public resolvers used when the system resolver is broken or
// filtered (Termux points at [::1]:53; some devices run DNS-level content
// filters that blocklist streaming CDNs).
var dnsServers = []string{"8.8.8.8:53", "1.1.1.1:53"}

// GetResolved fetches a URL while bypassing the system DNS resolver: the host
// is resolved via raw UDP DNS to public resolvers and pinned with curl's
// --resolve flag. Falls back to normal resolution if the lookup fails.
func GetResolved(ctx context.Context, rawURL string, headers map[string]string) (string, error) {
	host, port := hostPort(rawURL)
	if host == "" || net.ParseIP(host) != nil {
		return Get(ctx, rawURL, headers)
	}

	ip, err := dnsLookup(ctx, host)
	if err != nil {
		// System resolver may work fine on desktops - don't fail the request.
		return Get(ctx, rawURL, headers)
	}

	args := []string{"-s", "-L", "--resolve", net.JoinHostPort(host, port) + ":" + ip.String()}
	for k, v := range headers {
		args = append(args, "-H", k+": "+v)
	}
	args = append(args, rawURL)

	return runCurl(ctx, args)
}

// hostPort extracts the hostname and canonical port from a URL.
func hostPort(rawURL string) (string, string) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "", ""
	}
	host := u.Hostname()
	if host == "" {
		return "", ""
	}
	port := u.Port()
	if port == "" {
		if strings.EqualFold(u.Scheme, "https") {
			port = "443"
		} else {
			port = "80"
		}
	}
	return host, port
}

func runCurl(ctx context.Context, args []string) (string, error) {
	cmd := execCommandContext(ctx, "curl", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("curl failed: %w", err)
	}
	return string(output), nil
}

// dnsLookup resolves a hostname to an IPv4 address via raw UDP DNS.
func dnsLookup(ctx context.Context, host string) (net.IP, error) {
	query := buildDNSQuery(host)

	for _, server := range dnsServers {
		ip, err := dnsQueryServer(ctx, server, query)
		if err == nil {
			return ip, nil
		}
	}
	return nil, fmt.Errorf("dns lookup %q failed on all servers", host)
}

func dnsQueryServer(ctx context.Context, server string, query []byte) (net.IP, error) {
	conn, err := net.DialTimeout("udp", server, 3*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	} else {
		conn.SetDeadline(time.Now().Add(5 * time.Second))
	}

	if _, err := conn.Write(query); err != nil {
		return nil, err
	}

	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	return parseDNSResponse(buf[:n], query[0:2])
}

func buildDNSQuery(host string) []byte {
	// Header: random ID (a fixed one is spoofable), flags=standard query, 1 question
	var msg []byte
	id := make([]byte, 2)
	rand.Read(id)
	msg = append(msg, id...)
	msg = append(msg, 0x01, 0x00) // flags: standard query, recursion desired
	msg = append(msg, 0x00, 0x01) // 1 question
	msg = append(msg, 0x00, 0x00) // 0 answers
	msg = append(msg, 0x00, 0x00) // 0 authority
	msg = append(msg, 0x00, 0x00) // 0 additional

	for _, label := range strings.Split(host, ".") {
		msg = append(msg, byte(len(label)))
		msg = append(msg, []byte(label)...)
	}
	msg = append(msg, 0x00)       // root label
	msg = append(msg, 0x00, 0x01) // type A
	msg = append(msg, 0x00, 0x01) // class IN

	return msg
}

func parseDNSResponse(buf, txID []byte) (net.IP, error) {
	if len(buf) < 12 {
		return nil, fmt.Errorf("dns response too short")
	}
	// The response must echo our transaction ID and report success:
	// without this, a spoofed reply on a shared NAT pins the host.
	if buf[0] != txID[0] || buf[1] != txID[1] {
		return nil, fmt.Errorf("dns: transaction id mismatch")
	}
	if rcode := buf[3] & 0x0F; rcode != 0 {
		return nil, fmt.Errorf("dns: server error rcode=%d", rcode)
	}
	qdCount := int(buf[4])<<8 | int(buf[5])
	anCount := int(buf[6])<<8 | int(buf[7])
	if anCount == 0 {
		return nil, fmt.Errorf("dns: no answers")
	}

	offset := 12
	// Skip question section
	for i := 0; i < qdCount; i++ {
		for offset < len(buf) && buf[offset] != 0 {
			if buf[offset]&0xC0 == 0xC0 {
				offset += 2
				break
			}
			offset += int(buf[offset]) + 1
		}
		if offset < len(buf) && buf[offset] == 0 {
			offset++
		}
		offset += 4 // qtype + qclass
	}

	// Read answer records
	for i := 0; i < anCount && offset < len(buf); i++ {
		// Name (usually a compression pointer)
		if buf[offset]&0xC0 == 0xC0 {
			offset += 2
		} else {
			for offset < len(buf) && buf[offset] != 0 {
				offset += int(buf[offset]) + 1
			}
			offset++
		}
		if offset+10 > len(buf) {
			break
		}
		rtype := int(buf[offset])<<8 | int(buf[offset+1])
		rdlen := int(buf[offset+8])<<8 | int(buf[offset+9])
		offset += 10
		if rtype == 1 && rdlen == 4 && offset+4 <= len(buf) {
			return net.IPv4(buf[offset], buf[offset+1], buf[offset+2], buf[offset+3]), nil
		}
		offset += rdlen
	}
	return nil, fmt.Errorf("dns: no A record")
}
