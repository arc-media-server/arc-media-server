package transcode

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxGPUModelsUseTheExactRenderNodeAndPCIVendorDevice(t *testing.T) {
	root := t.TempDir()
	write := func(path, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	sysfs := filepath.Join(root, "drm")
	for node, values := range map[string][2]string{"renderD128": {"0x8086", "0x3e92"}, "renderD129": {"0x1002", "0x73bf"}, "renderD130": {"0x10de", "0x2504"}} {
		write(filepath.Join(sysfs, node, "device", "vendor"), values[0])
		write(filepath.Join(sysfs, node, "device", "device"), values[1])
	}
	db := filepath.Join(root, "pci.ids")
	write(db, "# Offline PCI fixture\n1002  AMD\n\t73bf  Navi 21 [Radeon RX 6800 XT]\n\t\t8086 3e92  Not an Intel adapter\n10de  NVIDIA\n\t2504  GA106 [GeForce RTX 3060]\n8086  Intel\n\t3e92  CoffeeLake-S GT2 [UHD Graphics 630]\nC 03  Display controller\n\t00  VGA\n")
	devices := []HardwareDevice{{Encoder: "h264_qsv", Device: "/dev/dri/renderD128"}, {Encoder: "h264_vaapi", Device: "/dev/dri/renderD129"}, {Encoder: "h264_nvenc"}}
	nodes := []string{"/dev/dri/renderD128", "/dev/dri/renderD129", "/dev/dri/renderD130"}
	names := linuxGPUDeviceNames(devices, nodes, sysfs, []string{filepath.Join(root, "missing"), db}, filepath.Join(root, "nvidia"))
	for i, want := range []string{"Intel UHD Graphics 630", "AMD Radeon RX 6800 XT", "NVIDIA GeForce RTX 3060"} {
		if names[i] != want {
			t.Fatalf("adapter%d=%q want%q", i, names[i], want)
		}
	}
	// Missing PCI data may not invent a model or borrow the other card's name.
	missing := linuxGPUDeviceNames(devices, nodes, sysfs, []string{filepath.Join(root, "missing")}, filepath.Join(root, "nvidia"))
	if len(missing) != 0 {
		t.Fatalf("guessed model: %+v", missing)
	}
	write(filepath.Join(sysfs, "renderD129", "device", "product_name"), "AMD Radeon RX 6800 XT\n")
	names = linuxGPUDeviceNames(devices, nodes, sysfs, nil, filepath.Join(root, "nvidia"))
	if names[1] != "AMD Radeon RX 6800 XT" {
		t.Fatalf("driver model missing: %+v", names)
	}
}

func TestNVIDIAProcModelIsUsedOnlyForOneUnambiguousGPU(t *testing.T) {
	root := t.TempDir()
	write := func(bus, model string) {
		t.Helper()
		dir := filepath.Join(root, bus)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "information"), []byte("Model: "+model+"\nGPU UUID: fixture-private-identifier\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	devices := []HardwareDevice{{Encoder: "h264_nvenc"}}
	write("0000:01:00.0", "NVIDIA GeForce RTX 3060")
	names := linuxGPUDeviceNames(devices, nil, filepath.Join(root, "missing"), nil, root)
	if names[0] != "NVIDIA GeForce RTX 3060" || strings.Contains(names[0], "private") {
		t.Fatalf("proc model: %+v", names)
	}
	write("0000:02:00.0", "NVIDIA GeForce RTX 4090")
	if names := linuxGPUDeviceNames(devices, nil, filepath.Join(root, "missing"), nil, root); len(names) != 0 {
		t.Fatalf("ambiguous default GPU named: %+v", names)
	}
}

func TestNativeGPUInventoryNamesAndAmbiguity(t *testing.T) {
	win := []byte(`[{"Name":"Intel(R) UHD Graphics 630","PNPDeviceID":"PCI\\VEN_8086&DEV_3E92"},{"Name":"NVIDIA GeForce RTX 3060","PNPDeviceID":"PCI\\VEN_10DE&DEV_2504"}]`)
	devices := []HardwareDevice{{Encoder: "h264_qsv"}, {Encoder: "h264_nvenc"}}
	names := adapterDeviceNames(devices, windowsGPUAdapters(win))
	if names[0] != "Intel(R) UHD Graphics 630" || names[1] != "NVIDIA GeForce RTX 3060" {
		t.Fatalf("Windows identity: %+v", names)
	}
	single := windowsGPUAdapters([]byte(`{"Name":"Intel GPU","PNPDeviceID":"PCI\\VEN_8086&DEV_1234"}`))
	if len(single) != 1 || single[0].vendor != "Intel" {
		t.Fatalf("single CIM object: %+v", single)
	}
	same := append(windowsGPUAdapters(win), gpuAdapter{vendor: "NVIDIA", name: "NVIDIA second GPU"})
	if names := adapterDeviceNames(devices, same); names[1] != "" || names[0] == "" {
		t.Fatalf("ambiguous native GPU: %+v", names)
	}
	apple := macGPUAdapters([]byte(`{"SPDisplaysDataType":[{"sppci_model":"Apple M3","_name":"ignored fallback"}]}`))
	if names := adapterDeviceNames([]HardwareDevice{{Encoder: "h264_videotoolbox"}}, apple); names[0] != "Apple M3" {
		t.Fatalf("Apple identity: %+v", names)
	}
	apple = append(apple, gpuAdapter{name: "External GPU"})
	if names := adapterDeviceNames([]HardwareDevice{{Encoder: "h264_videotoolbox"}}, apple); len(names) != 0 {
		t.Fatalf("guessed OS-selected adapter: %+v", names)
	}
	if len(windowsGPUAdapters([]byte("unavailable"))) != 0 || len(macGPUAdapters([]byte("unavailable"))) != 0 {
		t.Fatal("invalid inventory accepted")
	}
}

func TestHardwareNamesAreAdditiveAndCannotChangeSelection(t *testing.T) {
	m := New(t.TempDir(), func() string { return "fixture" }, func() string { return "fixture" }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.renderNodes = func() []string { return []string{"/dev/dri/renderD128"} }
	m.probeHardware = func(_ context.Context, _ string, args []string, _ string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "-filters") || strings.Contains(strings.Join(args, " "), "h264_qsv") {
			return nil, nil
		}
		return []byte("Nothing was written"), nil
	}
	m.gpuNames = func(_ context.Context, devices []HardwareDevice, nodes []string) map[int]string {
		if len(devices) != 1 || len(nodes) != 1 {
			t.Fatalf("inventory changed: %+v %+v", devices, nodes)
		}
		return map[int]string{0: "Intel UHD Graphics 630\n", 99: "invalid index"}
	}
	m.UseDevice("h264_qsv:/dev/dri/renderD128")
	caps := m.Detect(t.Context())
	if len(caps.Devices) != 1 || caps.Devices[0].Name != "Intel UHD Graphics 630" || caps.Devices[0].ID != "h264_qsv:/dev/dri/renderD128" || caps.HWLabel != "Intel Quick Sync" || caps.HWEncoder != "h264_qsv" {
		t.Fatalf("identity changed encoding: %+v", caps)
	}
}

func TestGPUNameBoundsPreserveUnicode(t *testing.T) {
	got := cleanGPUName(strings.Repeat("界", 200) + "\n")
	if len([]rune(got)) != 160 || strings.ContainsAny(got, "\n\r\x1b") {
		t.Fatalf("unbounded/unsafe name: %q", got)
	}
}
