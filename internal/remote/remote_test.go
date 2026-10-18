package remote

import (
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestListenerServesHTTPAndHTTPS(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	// A throwaway certificate from httptest.
	ts := httptest.NewUnstartedServer(nil)
	ts.StartTLS()
	cert := ts.TLS.Certificates[0]
	ts.Close()
	cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS != nil {
			io.WriteString(w, "secure")
		} else {
			io.WriteString(w, "plain")
		}
	})}
	go srv.Serve(Listener(ln, func() *tls.Config { return cfg }))
	defer srv.Close()

	addr := ln.Addr().String()
	get := func(url string) string {
		c := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
		resp, err := c.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	if got := get("http://" + addr + "/"); got != "plain" {
		t.Errorf("http: got %q", got)
	}
	if got := get("https://" + addr + "/"); got != "secure" {
		t.Errorf("https: got %q", got)
	}
}

func TestMapperForwardsPort(t *testing.T) {
	var mu sync.Mutex
	var actions []string
	var mux http.ServeMux
	srv := httptest.NewServer(&mux)
	defer srv.Close()
	mux.HandleFunc("/desc.xml", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `<?xml version="1.0"?><root xmlns="urn:schemas-upnp-org:device-1-0"><device><friendlyName>Test Router</friendlyName>
<deviceList><device><deviceList><device><serviceList><service><serviceType>urn:schemas-upnp-org:service:WANIPConnection:1</serviceType>
<controlURL>/ctl</controlURL></service></serviceList></device></deviceList></device></deviceList></device></root>`)
	})
	mux.HandleFunc("/ctl", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		action := r.Header.Get("SOAPAction")
		mu.Lock()
		actions = append(actions, action)
		mu.Unlock()
		switch {
		case strings.Contains(action, "AddPortMapping"):
			if !strings.Contains(string(b), "<NewExternalPort>32400</NewExternalPort>") || !strings.Contains(string(b), "<NewInternalPort>8099</NewInternalPort>") {
				t.Errorf("unexpected AddPortMapping body: %s", b)
			}
			io.WriteString(w, `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:AddPortMappingResponse/></s:Body></s:Envelope>`)
		case strings.Contains(action, "GetExternalIPAddress"):
			io.WriteString(w, `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetExternalIPAddressResponse xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1"><NewExternalIPAddress>203.0.113.7</NewExternalIPAddress></u:GetExternalIPAddressResponse></s:Body></s:Envelope>`)
		default:
			w.WriteHeader(500)
			io.WriteString(w, `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><s:Fault><detail><UPnPError><errorCode>401</errorCode><errorDescription>Invalid Action</errorDescription></UPnPError></detail></s:Fault></s:Body></s:Envelope>`)
		}
	})
	t.Setenv("AMS_UPNP_URL", srv.URL+"/desc.xml")
	m := NewMapper(slog.New(slog.NewTextHandler(io.Discard, nil)))
	gw, ip, err := m.mapPort(context.Background(), 8099, 32400)
	if err != nil {
		t.Fatal(err)
	}
	if gw.name != "Test Router" || ip != "203.0.113.7" {
		t.Errorf("got router %q, ip %q", gw.name, ip)
	}
	if err := gw.soap(context.Background(), "Nonsense", "", nil); err == nil || !strings.Contains(err.Error(), "Invalid Action") {
		t.Errorf("fault not reported: %v", err)
	}
}
