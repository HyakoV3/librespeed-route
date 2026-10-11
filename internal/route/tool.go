package route

import (
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
)

// Spec is the program to run and its arguments, as they are given to the system (no shell is ever in between).
type Spec struct {
	Tool string   // "tracert", "traceroute" or "tracepath": the words of Parse
	Path string   // absolute
	Args []string // the last one is the address, never a name
}

// ErrNoTool is returned when the system has none of the tools.
var ErrNoTool = errors.New("no tracert, traceroute or tracepath was found in the places where the system keeps them")

const (
	maxHops = "30"
)

// CommandFor builds the command for the system goos and the address ip. exists tells whether a file is there (os.Stat in the program, a
// list in the tests); systemRoot is %SystemRoot% of Windows. The paths are fixed places, not PATH.
func CommandFor(goos string, ip netip.Addr, exists func(string) bool, systemRoot string) (Spec, error) {
	address := ip.WithZone("").String()
	v := "-4"
	if ip.Is6() && !ip.Is4In6() {
		v = "-6"
	}
	switch goos {
	case "windows":
		if systemRoot == "" {
			systemRoot = `C:\Windows`
		}
		path := strings.TrimRight(systemRoot, `\/`) + `\System32\tracert.exe`
		if !exists(path) {
			return Spec{}, ErrNoTool
		}
		return Spec{"tracert", path, []string{"-d", "-h", maxHops, "-w", "1000", v, address}}, nil
	case "darwin", "freebsd", "openbsd", "netbsd", "dragonfly":
		name := "traceroute"
		if v == "-6" {
			name = "traceroute6"
		}
		path := "/usr/sbin/" + name
		if !exists(path) {
			return Spec{}, ErrNoTool
		}
		return Spec{"traceroute", path, []string{"-n", "-m", maxHops, "-w", "1", address}}, nil
	default:
		for _, dir := range []string{"/usr/bin", "/usr/sbin", "/bin", "/sbin", "/usr/local/bin"} {
			if p := dir + "/traceroute"; exists(p) {
				return Spec{"traceroute", p, []string{"-n", "-m", maxHops, "-w", "1", v, address}}, nil
			}
		}
		for _, dir := range []string{"/usr/bin", "/usr/sbin", "/bin", "/sbin"} {
			if p := dir + "/tracepath"; exists(p) {
				return Spec{"tracepath", p, []string{"-n", "-m", maxHops, v, address}}, nil
			}
		}
		return Spec{}, ErrNoTool
	}
}

// FileExists is the check of the real system.
func FileExists(path string) bool {
	if !filepath.IsAbs(path) && !strings.HasPrefix(path, `C:\`) && !(len(path) > 2 && path[1] == ':') {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
