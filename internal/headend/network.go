// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package headend

import (
	"net"
	"sort"
)

type PlaybackEndpoint struct {
	IP          string `json:"ip"`
	PlaylistURL string `json:"playlist_url"`
}

// PlaybackEndpoints lists addresses clients can use instead of a wildcard bind
// address. Multiple active networks are listed rather than guessing a route.
func PlaybackEndpoints(listen string) ([]PlaybackEndpoint, error) {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return nil, err
	}
	if ip := net.ParseIP(host); ip != nil && !ip.IsUnspecified() {
		return playbackEndpoints(listen, nil), nil
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			return nil, err
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil {
				ips = append(ips, ip)
			}
		}
	}
	return playbackEndpoints(listen, ips), nil
}

func playbackEndpoints(listen string, ips []net.IP) []PlaybackEndpoint {
	host, port, _ := net.SplitHostPort(listen)
	bindIP := net.ParseIP(host)
	addresses := map[string]bool{}
	if bindIP != nil && !bindIP.IsUnspecified() {
		addresses[bindIP.String()] = true
	} else {
		for _, ip := range ips {
			if ip.IsGlobalUnicast() && (ip.To4() != nil) == (bindIP.To4() != nil) {
				addresses[ip.String()] = true
			}
		}
		if len(addresses) == 0 {
			if bindIP.To4() != nil {
				addresses["127.0.0.1"] = true
			} else {
				addresses["::1"] = true
			}
		}
	}
	var endpoints []PlaybackEndpoint
	for address := range addresses {
		endpoints = append(endpoints, PlaybackEndpoint{IP: address, PlaylistURL: "http://" + net.JoinHostPort(address, port) + "/lineup.m3u"})
	}
	sort.Slice(endpoints, func(i, j int) bool {
		a, b := net.ParseIP(endpoints[i].IP), net.ParseIP(endpoints[j].IP)
		if a.IsPrivate() != b.IsPrivate() {
			return a.IsPrivate()
		}
		return endpoints[i].IP < endpoints[j].IP
	})
	return endpoints
}
