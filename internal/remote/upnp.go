package remote

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// UPnP port mapping: asking the router to forward a port to this computer, as Plex does when
// remote access is turned on. Only the small part of UPnP IGD that's needed is spoken here.

// Mapper keeps a port forwarded on the router while it's wanted.
type Mapper struct {
	log  *slog.Logger
	wake chan struct{}

	mu       sync.Mutex
	enabled  bool
	internal int
	external int
	status   MapStatus
}

// MapStatus is what Settings shows about the router.
type MapStatus struct {
	// off, working, mapped or failed.
	State string `json:"state"`
	// The router's name, and the home's outside address it reports.
	Router     string `json:"router,omitempty"`
	ExternalIP string `json:"external_ip,omitempty"`
	Error      string `json:"error,omitempty"`
}

// The router keeps the forwarding for an hour; it's renewed every half hour.
const (
	leaseSeconds = 3600
	renewEvery   = 30 * time.Minute
	retryEvery   = 5 * time.Minute
	description  = "Arc Media Server"
)

func NewMapper(log *slog.Logger) *Mapper {
	return &Mapper{log: log, wake: make(chan struct{}, 1), status: MapStatus{State: "off"}}
}

// Configure says whether to forward the external port to the internal one.
func (m *Mapper) Configure(enabled bool, internal, external int) {
	m.mu.Lock()
	changed := enabled != m.enabled || internal != m.internal || external != m.external
	m.enabled, m.internal, m.external = enabled, internal, external
	m.mu.Unlock()
	if changed {
		select {
		case m.wake <- struct{}{}:
		default:
		}
	}
}

// Status reports the forwarding.
func (m *Mapper) Status() MapStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *Mapper) setStatus(s MapStatus) {
	m.mu.Lock()
	m.status = s
	m.mu.Unlock()
}

// Run keeps the forwarding in place until ctx ends, then removes it.
func (m *Mapper) Run(ctx context.Context) {
	var mapped *gateway // where a forwarding is in place
	var mappedExt int
	unmap := func() {
		if mapped == nil {
			return
		}
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		if err := mapped.deletePortMapping(c, mappedExt); err != nil {
			m.log.Warn("couldn't remove the port forwarding from the router", "err", err)
		} else {
			m.log.Info("port forwarding removed from the router", "port", mappedExt)
		}
		cancel()
		mapped = nil
	}
	defer unmap()
	for {
		m.mu.Lock()
		enabled, internal, external := m.enabled, m.internal, m.external
		m.mu.Unlock()
		wait := renewEvery
		if !enabled {
			unmap()
			m.setStatus(MapStatus{State: "off"})
		} else {
			if mapped != nil && mappedExt != external {
				unmap()
			}
			if mapped == nil {
				m.setStatus(MapStatus{State: "working"})
			}
			gw, ip, err := m.mapPort(ctx, internal, external)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				m.log.Warn("couldn't forward the port on the router", "err", err)
				m.setStatus(MapStatus{State: "failed", Error: err.Error()})
				wait = retryEvery
			} else {
				if mapped == nil {
					m.log.Info("port forwarded on the router", "router", gw.name, "external_port", external, "address", ip)
				}
				mapped, mappedExt = gw, external
				m.setStatus(MapStatus{State: "mapped", Router: gw.name, ExternalIP: ip})
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-m.wake:
		case <-time.After(wait):
		}
	}
}

func (m *Mapper) mapPort(ctx context.Context, internal, external int) (*gateway, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	gw, err := discover(ctx)
	if err != nil {
		return nil, "", err
	}
	if err := gw.addPortMapping(ctx, internal, external); err != nil {
		return nil, "", err
	}
	ip, _ := gw.externalIP(ctx)
	return gw, ip, nil
}

// gateway is the router's WAN connection service.
type gateway struct {
	name        string
	controlURL  string
	serviceType string
	localIP     string // this computer's address as the router sees it
}

// discover finds the router with SSDP (or at AMS_UPNP_URL, a description URL, for testing).
func discover(ctx context.Context) (*gateway, error) {
	if u := os.Getenv("AMS_UPNP_URL"); u != "" {
		return describe(ctx, u)
	}
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	dst := &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}
	for _, st := range []string{"urn:schemas-upnp-org:device:InternetGatewayDevice:2", "urn:schemas-upnp-org:device:InternetGatewayDevice:1"} {
		msg := "M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nST: " + st + "\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\n\r\n"
		if _, err := conn.WriteTo([]byte(msg), dst); err != nil {
			return nil, err
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	conn.SetReadDeadline(deadline)
	buf := make([]byte, 2048)
	tried := map[string]bool{}
	var lastErr error
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			break
		}
		for _, line := range strings.Split(string(buf[:n]), "\r\n") {
			if k, v, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(k), "location") {
				loc := strings.TrimSpace(v)
				if tried[loc] {
					continue
				}
				tried[loc] = true
				gw, err := describe(ctx, loc)
				if err == nil {
					return gw, nil
				}
				lastErr = err
			}
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("no router answered. Turn on UPnP in your router's settings, or forward the port by hand")
}

// The parts of a UPnP device description that matter.
type upnpDevice struct {
	FriendlyName string        `xml:"friendlyName"`
	ModelName    string        `xml:"modelName"`
	Services     []upnpService `xml:"serviceList>service"`
	Devices      []upnpDevice  `xml:"deviceList>device"`
}

type upnpService struct {
	ServiceType string `xml:"serviceType"`
	ControlURL  string `xml:"controlURL"`
}

// describe reads the router's description and finds its WAN connection service.
func describe(ctx context.Context, location string) (*gateway, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("the router didn't answer: %w", err)
	}
	defer resp.Body.Close()
	var root struct {
		URLBase string     `xml:"URLBase"`
		Device  upnpDevice `xml:"device"`
	}
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&root); err != nil {
		return nil, fmt.Errorf("the router's description couldn't be read: %w", err)
	}
	var find func(d upnpDevice) *upnpService
	find = func(d upnpDevice) *upnpService {
		for i, s := range d.Services {
			if strings.Contains(s.ServiceType, ":WANIPConnection:") || strings.Contains(s.ServiceType, ":WANPPPConnection:") {
				return &d.Services[i]
			}
		}
		for _, sub := range d.Devices {
			if s := find(sub); s != nil {
				return s
			}
		}
		return nil
	}
	svc := find(root.Device)
	if svc == nil {
		return nil, errors.New("the router doesn't offer port forwarding over UPnP")
	}
	base, err := url.Parse(location)
	if err != nil {
		return nil, err
	}
	if root.URLBase != "" {
		if b, err := url.Parse(root.URLBase); err == nil {
			base = b
		}
	}
	control, err := base.Parse(svc.ControlURL)
	if err != nil {
		return nil, err
	}
	// This computer's address on the router's network.
	conn, err := net.Dial("udp4", base.Host)
	if err != nil && !strings.Contains(base.Host, ":") {
		conn, err = net.Dial("udp4", base.Host+":80")
	}
	if err != nil {
		return nil, err
	}
	local := conn.LocalAddr().(*net.UDPAddr).IP.String()
	conn.Close()
	name := root.Device.FriendlyName
	if name == "" {
		name = root.Device.ModelName
	}
	return &gateway{name: name, controlURL: control.String(), serviceType: svc.ServiceType, localIP: local}, nil
}

// soap calls an action on the router; out receives the response body's action element.
func (g *gateway) soap(ctx context.Context, action, args string, out any) error {
	body := `<?xml version="1.0"?>` +
		`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">` +
		`<s:Body><u:` + action + ` xmlns:u="` + g.serviceType + `">` + args + `</u:` + action + `></s:Body></s:Envelope>`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.controlURL, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPAction", `"`+g.serviceType+"#"+action+`"`)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	var env struct {
		Body struct {
			Inner []byte `xml:",innerxml"`
			Fault struct {
				Code        int    `xml:"detail>UPnPError>errorCode"`
				Description string `xml:"detail>UPnPError>errorDescription"`
			} `xml:"Fault"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("the router's answer couldn't be read (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return &upnpError{Code: env.Body.Fault.Code, Description: env.Body.Fault.Description, Status: resp.StatusCode}
	}
	if out != nil {
		return xml.Unmarshal(env.Body.Inner, out)
	}
	return nil
}

type upnpError struct {
	Code        int
	Description string
	Status      int
}

func (e *upnpError) Error() string {
	switch e.Code {
	case 718:
		return "the router already forwards that port to another device; choose another outside port"
	case 606, 501:
		return "the router refused to forward the port (UPnP may be read-only in its settings)"
	}
	if e.Description != "" {
		return fmt.Sprintf("the router refused: %s (%d)", e.Description, e.Code)
	}
	return fmt.Sprintf("the router refused (HTTP %d)", e.Status)
}

func (g *gateway) addPortMapping(ctx context.Context, internal, external int) error {
	add := func(lease int) error {
		return g.soap(ctx, "AddPortMapping", fmt.Sprintf(
			"<NewRemoteHost></NewRemoteHost><NewExternalPort>%d</NewExternalPort><NewProtocol>TCP</NewProtocol>"+
				"<NewInternalPort>%d</NewInternalPort><NewInternalClient>%s</NewInternalClient><NewEnabled>1</NewEnabled>"+
				"<NewPortMappingDescription>%s</NewPortMappingDescription><NewLeaseDuration>%d</NewLeaseDuration>",
			external, internal, g.localIP, description, lease), nil)
	}
	err := add(leaseSeconds)
	var ue *upnpError
	if errors.As(err, &ue) && ue.Code == 725 {
		err = add(0) // the router only keeps permanent forwardings; it's removed when Arc stops
	}
	return err
}

func (g *gateway) deletePortMapping(ctx context.Context, external int) error {
	return g.soap(ctx, "DeletePortMapping", fmt.Sprintf(
		"<NewRemoteHost></NewRemoteHost><NewExternalPort>%d</NewExternalPort><NewProtocol>TCP</NewProtocol>", external), nil)
}

func (g *gateway) externalIP(ctx context.Context) (string, error) {
	var out struct {
		IP string `xml:"NewExternalIPAddress"`
	}
	err := g.soap(ctx, "GetExternalIPAddress", "", &out)
	return strings.TrimSpace(out.IP), err
}
