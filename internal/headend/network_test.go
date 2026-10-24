// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package headend

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestPlaybackAddressesMatchListener(t *testing.T) {
	var ips []net.IP
	for _, value := range []string{"127.0.0.1", "169.254.1.2", "224.0.0.1", "0.0.0.0", "192.168.1.20", "10.0.0.20", "192.168.1.20", "2001:db8::20", "fe80::20"} {
		ips = append(ips, net.ParseIP(value))
	}
	for _, test := range []struct {
		listen string
		ips    []net.IP
		want   []PlaybackEndpoint
	}{
		{"0.0.0.0:8090", ips, []PlaybackEndpoint{
			{IP: "10.0.0.20", PlaylistURL: "http://10.0.0.20:8090/lineup.m3u"},
			{IP: "192.168.1.20", PlaylistURL: "http://192.168.1.20:8090/lineup.m3u"},
		}},
		{"192.168.1.20:9000", ips, []PlaybackEndpoint{{IP: "192.168.1.20", PlaylistURL: "http://192.168.1.20:9000/lineup.m3u"}}},
		{"127.0.0.1:8090", ips, []PlaybackEndpoint{{IP: "127.0.0.1", PlaylistURL: "http://127.0.0.1:8090/lineup.m3u"}}},
		{"[::]:8090", ips, []PlaybackEndpoint{{IP: "2001:db8::20", PlaylistURL: "http://[2001:db8::20]:8090/lineup.m3u"}}},
		{"0.0.0.0:8090", nil, []PlaybackEndpoint{{IP: "127.0.0.1", PlaylistURL: "http://127.0.0.1:8090/lineup.m3u"}}},
		{"[::]:8090", nil, []PlaybackEndpoint{{IP: "::1", PlaylistURL: "http://[::1]:8090/lineup.m3u"}}},
	} {
		t.Run(test.listen, func(t *testing.T) {
			if got := playbackEndpoints(test.listen, test.ips); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestServedLineupUsesReachableSocketAddressOrConfiguredBase(t *testing.T) {
	for _, base := range []string{"", "https://tv.example/iptv"} {
		t.Run(base, func(t *testing.T) {
			c := DefaultConfig()
			c.BaseURL = base
			c.Muxes = []Mux{sampleMux()}
			e := Engine{Config: c}
			server := httptest.NewServer(e.Handler())
			defer server.Close()
			request, err := http.NewRequest(http.MethodGet, server.URL+"/lineup.m3u", nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Host = "untrusted.example"
			request.Header.Set("X-Forwarded-Host", "untrusted.example")
			request.Header.Set("X-Forwarded-Proto", "https")
			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			data, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			wantBase := base
			if wantBase == "" {
				wantBase = server.URL
			}
			if response.StatusCode != http.StatusOK || !strings.Contains(string(data), wantBase+"/hls/local/3/index.m3u8") || strings.Contains(string(data), "untrusted.example") {
				t.Fatalf("unexpected lineup (%d): %s", response.StatusCode, data)
			}
		})
	}
}
