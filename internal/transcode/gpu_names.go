package transcode

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
)

// Adapter identity is cosmetic. Missing metadata never changes encode admission, ordering or
// the saved encoder/device ID. Ambiguous unbound APIs must not borrow another GPU's model.
type gpuAdapter struct{ node, vendor, name string }

func cleanGPUName(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) > 160 {
		r = r[:160]
	}
	return string(r)
}
func gpuMaker(id string) string {
	switch strings.TrimPrefix(strings.ToLower(strings.TrimSpace(id)), "0x") {
	case "8086":
		return "Intel"
	case "1002":
		return "AMD"
	case "10de":
		return "NVIDIA"
	}
	return ""
}
func encoderMaker(encoder string) string {
	switch encoder {
	case "h264_qsv":
		return "Intel"
	case "h264_nvenc":
		return "NVIDIA"
	case "h264_amf":
		return "AMD"
	}
	return ""
}
func adapterDeviceNames(devices []HardwareDevice, adapters []gpuAdapter) map[int]string {
	names := map[int]string{}
	for i, d := range devices {
		if d.Device != "" {
			for _, a := range adapters {
				if a.node == d.Device && a.name != "" {
					names[i] = a.name
					break
				}
			}
			continue
		}
		candidates := []gpuAdapter{}
		for _, a := range adapters {
			if (d.Encoder == "h264_videotoolbox") || (encoderMaker(d.Encoder) != "" && a.vendor == encoderMaker(d.Encoder)) {
				candidates = append(candidates, a)
			}
		}
		if len(candidates) == 1 && candidates[0].name != "" {
			names[i] = candidates[0].name
		}
	}
	return names
}
func systemGPUDeviceNames(ctx context.Context, devices []HardwareDevice, nodes []string) map[int]string {
	switch runtime.GOOS {
	case "linux":
		return linuxGPUDeviceNames(devices, nodes, "/sys/class/drm", []string{"/usr/share/misc/pci.ids", "/usr/share/hwdata/pci.ids"}, "/proc/driver/nvidia/gpus")
	case "windows":
		out := gpuInventoryCommand(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Get-CimInstance Win32_VideoController | Select-Object Name,PNPDeviceID | ConvertTo-Json -Compress")
		return adapterDeviceNames(devices, windowsGPUAdapters(out))
	case "darwin":
		out := gpuInventoryCommand(ctx, "/usr/sbin/system_profiler", "SPDisplaysDataType", "-json")
		return adapterDeviceNames(devices, macGPUAdapters(out))
	}
	return nil
}
func gpuInventoryCommand(ctx context.Context, bin string, args ...string) []byte {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.WaitDelay = 250 * time.Millisecond
	out := &probeOutput{}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Run(); err != nil {
		return nil
	}
	return out.Bytes()
}
func windowsGPUAdapters(raw []byte) []gpuAdapter {
	type controller struct {
		Name        string
		PNPDeviceID string
	}
	var all []controller
	if json.Unmarshal(raw, &all) != nil {
		var one controller
		if json.Unmarshal(raw, &one) != nil {
			return nil
		}
		all = []controller{one}
	}
	if len(all) > 64 {
		return nil
	}
	adapters := []gpuAdapter{}
	for _, c := range all {
		_, rest, ok := strings.Cut(strings.ToUpper(c.PNPDeviceID), "VEN_")
		vendor := ""
		if ok && len(rest) >= 4 {
			vendor = gpuMaker(rest[:4])
		}
		adapters = append(adapters, gpuAdapter{vendor: vendor, name: cleanGPUName(c.Name)})
	}
	return adapters
}
func macGPUAdapters(raw []byte) []gpuAdapter {
	var info struct {
		Displays []struct {
			Model string `json:"sppci_model"`
			Name  string `json:"_name"`
		} `json:"SPDisplaysDataType"`
	}
	if json.Unmarshal(raw, &info) != nil || len(info.Displays) > 64 {
		return nil
	}
	adapters := []gpuAdapter{}
	for _, d := range info.Displays {
		name := d.Model
		if name == "" {
			name = d.Name
		}
		adapters = append(adapters, gpuAdapter{name: cleanGPUName(name)})
	}
	return adapters
}
func readGPUAttribute(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 1024))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}
func linuxGPUDeviceNames(devices []HardwareDevice, nodes []string, sysfs string, databases []string, nvidiaRoot string) map[int]string {
	if len(nodes) > 64 {
		return nil
	}
	adapters := []gpuAdapter{}
	ids := []string{}
	for _, node := range nodes {
		base := filepath.Join(sysfs, filepath.Base(node), "device")
		vendor := strings.TrimPrefix(strings.ToLower(readGPUAttribute(filepath.Join(base, "vendor"))), "0x")
		device := strings.TrimPrefix(strings.ToLower(readGPUAttribute(filepath.Join(base, "device"))), "0x")
		name := cleanGPUName(readGPUAttribute(filepath.Join(base, "product_name")))
		adapters = append(adapters, gpuAdapter{node: node, vendor: gpuMaker(vendor), name: name})
		ids = append(ids, vendor+":"+device)
	}
	wanted := map[string]string{}
	for _, id := range ids {
		wanted[id] = ""
	}
	for _, path := range databases {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		readPCIGPUNames(io.LimitReader(f, 4<<20), wanted)
		f.Close()
	}
	for i := range adapters {
		if adapters[i].name == "" {
			adapters[i].name = wanted[ids[i]]
		}
	}
	// NVENC may expose no DRM render node in Docker. Read only the model from a unique
	// NVIDIA proc entry, never its UUID or serial number; multiple cards remain ambiguous.
	nvidia := []int{}
	for i, a := range adapters {
		if a.vendor == "NVIDIA" {
			nvidia = append(nvidia, i)
		}
	}
	if len(nvidia) == 0 || (len(nvidia) == 1 && adapters[nvidia[0]].name == "") {
		paths, _ := filepath.Glob(filepath.Join(nvidiaRoot, "*", "information"))
		if len(paths) == 1 {
			f, err := os.Open(paths[0])
			if err == nil {
				scanner := bufio.NewScanner(io.LimitReader(f, 16<<10))
				for scanner.Scan() {
					if model, ok := strings.CutPrefix(scanner.Text(), "Model:"); ok {
						a := gpuAdapter{vendor: "NVIDIA", name: cleanGPUName(model)}
						if len(nvidia) == 0 {
							adapters = append(adapters, a)
						} else {
							adapters[nvidia[0]].name = a.name
						}
						break
					}
				}
				f.Close()
			}
		}
	}
	names := adapterDeviceNames(devices, adapters)
	paths, _ := filepath.Glob(filepath.Join(nvidiaRoot, "*", "information"))
	if len(paths) > 1 {
		for i, d := range devices {
			if d.Encoder == "h264_nvenc" && d.Device == "" {
				delete(names, i)
			}
		}
	}
	return names
}

// Parse just vendor/device names; subsystem IDs and class tables are not GPU identities.
func readPCIGPUNames(r io.Reader, wanted map[string]string) {
	scanner := bufio.NewScanner(r)
	vendor := ""
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line[0] != '\t' {
			fields := strings.Fields(line)
			vendor = ""
			if len(fields) > 1 && len(fields[0]) == 4 {
				vendor = strings.ToLower(fields[0])
			}
			continue
		}
		if strings.HasPrefix(line, "\t\t") || vendor == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || len(fields[0]) != 4 {
			continue
		}
		id := vendor + ":" + strings.ToLower(fields[0])
		if _, ok := wanted[id]; !ok || wanted[id] != "" {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), fields[0]))
		if start := strings.Index(name, "["); start >= 0 {
			if end := strings.LastIndex(name, "]"); end > start {
				name = name[start+1 : end]
			}
		}
		maker := gpuMaker(vendor)
		if maker != "" && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(maker)) {
			name = maker + " " + name
		}
		wanted[id] = cleanGPUName(name)
	}
}
