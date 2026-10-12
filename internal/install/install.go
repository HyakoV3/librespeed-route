// Package install puts the program where the browsers look for a native messaging host, and takes it away again: the program is copied to
// a folder of the user, a manifest is written for each browser family (Chromium: Chrome, Chromium and Edge; Firefox), and on Windows a key
// of the registry of the user points at the manifest. Nothing needs administrator rights, nothing outside the folders and keys of the
// user is touched, and uninstall removes exactly what was written.
package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/HyakoV3/librespeed-route/internal/nativehost"
)

// Registry is the part of the registry of Windows that is used (the default value of a key of the user).
type Registry interface {
	Set(key, value string) error
	Get(key string) (string, bool)
	Delete(key string) error
}

// Env is what the installer takes from the system.
type Env struct {
	GOOS         string
	Home         string
	LocalAppData string // Windows: %LOCALAPPDATA%
	XDGDataHome  string // Linux: $XDG_DATA_HOME
	Exe          string // the program that is running, to copy
	Registry     Registry
	Out          io.Writer
}

type browser struct {
	Name   string
	Family string // "chromium" or "firefox"
	// Root: the folder that exists when the browser has been used; Dir: the folder of the manifests
	Root, Dir string
	Key       string // Windows: the key under HKCU
}

// browsers are the places that each system has, for the user. An empty Dir means that the system has none.
func browsers(env Env) []browser {
	home := env.Home
	switch env.GOOS {
	case "windows":
		return []browser{
			{Name: "Chrome", Family: "chromium", Key: `Software\Google\Chrome\NativeMessagingHosts\` + nativehost.HostName},
			{Name: "Chromium", Family: "chromium", Key: `Software\Chromium\NativeMessagingHosts\` + nativehost.HostName},
			{Name: "Edge", Family: "chromium", Key: `Software\Microsoft\Edge\NativeMessagingHosts\` + nativehost.HostName},
			{Name: "Firefox", Family: "firefox", Key: `Software\Mozilla\NativeMessagingHosts\` + nativehost.HostName},
		}
	case "darwin":
		app := filepath.Join(home, "Library", "Application Support")
		return []browser{
			{"Chrome", "chromium", filepath.Join(app, "Google", "Chrome"), filepath.Join(app, "Google", "Chrome", "NativeMessagingHosts"), ""},
			{"Chromium", "chromium", filepath.Join(app, "Chromium"), filepath.Join(app, "Chromium", "NativeMessagingHosts"), ""},
			{"Edge", "chromium", filepath.Join(app, "Microsoft Edge"), filepath.Join(app, "Microsoft Edge", "NativeMessagingHosts"), ""},
			{"Firefox", "firefox", filepath.Join(app, "Mozilla"), filepath.Join(app, "Mozilla", "NativeMessagingHosts"), ""},
		}
	default:
		cfg := filepath.Join(home, ".config")
		return []browser{
			{"Chrome", "chromium", filepath.Join(cfg, "google-chrome"), filepath.Join(cfg, "google-chrome", "NativeMessagingHosts"), ""},
			{"Chromium", "chromium", filepath.Join(cfg, "chromium"), filepath.Join(cfg, "chromium", "NativeMessagingHosts"), ""},
			{"Edge", "chromium", filepath.Join(cfg, "microsoft-edge"), filepath.Join(cfg, "microsoft-edge", "NativeMessagingHosts"), ""},
			{"Firefox", "firefox", filepath.Join(home, ".mozilla"), filepath.Join(home, ".mozilla", "native-messaging-hosts"), ""},
		}
	}
}

// ProgramDir is the folder of the user where the program is kept.
func ProgramDir(env Env) string {
	switch env.GOOS {
	case "windows":
		base := env.LocalAppData
		if base == "" {
			base = filepath.Join(env.Home, "AppData", "Local")
		}
		return filepath.Join(base, "librespeed-route")
	case "darwin":
		return filepath.Join(env.Home, "Library", "Application Support", "librespeed-route")
	}
	base := env.XDGDataHome
	if base == "" {
		base = filepath.Join(env.Home, ".local", "share")
	}
	return filepath.Join(base, "librespeed-route")
}

func binaryName(goos string) string {
	if goos == "windows" {
		return "librespeed-route.exe"
	}
	return "librespeed-route"
}

// ErrPath is returned when the folder has a character that cmd.exe (which starts the host for a Chromium browser on Windows) would read.
var ErrPath = errors.New(`the folder of the program has a character (% " ^ & !) that Windows would read as part of a command: use another user, or a folder without them`)

// Manifest is the file that a browser reads: where the host is and who may start it.
type Manifest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Path           string   `json:"path"`
	Type           string   `json:"type"`
	AllowedOrigins []string `json:"allowed_origins,omitempty"`
	AllowedExts    []string `json:"allowed_extensions,omitempty"`
}

// ManifestFor is the manifest of a family of browsers for the program at path.
func ManifestFor(family, path string) ([]byte, error) {
	m := Manifest{
		Name:        nativehost.HostName,
		Description: "librespeed-route: traces the route from this computer to the server of the page that you clicked on, for the route page of LibreSpeed",
		Path:        path,
		Type:        "stdio",
	}
	if family == "firefox" {
		m.AllowedExts = append([]string{}, nativehost.GeckoIDs...)
	} else {
		for _, id := range nativehost.ChromeIDs {
			m.AllowedOrigins = append(m.AllowedOrigins, "chrome-extension://"+id+"/")
		}
	}
	out, err := json.MarshalIndent(m, "", "  ")
	return append(out, '\n'), err
}

func writeFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func copyFile(from, to string) error {
	if a, err := filepath.Abs(from); err == nil {
		if b, err := filepath.Abs(to); err == nil && a == b {
			return nil
		}
	}
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return writeFile(to, data, 0o755)
}

// Install copies the program and writes the manifests. With all, every browser of the system gets one, used or not; otherwise only the ones
// whose folder exists (they have been used) and, on Windows, every key (the registry is cheap and a browser that is installed later finds it).
func Install(env Env, all bool) error {
	dir := ProgramDir(env)
	if env.GOOS == "windows" && strings.ContainsAny(dir, `%"^&!`) {
		return ErrPath
	}
	if env.Exe == "" {
		return errors.New("the path of the running program is not known")
	}
	program := filepath.Join(dir, binaryName(env.GOOS))
	if err := copyFile(env.Exe, program); err != nil {
		return fmt.Errorf("copying the program: %w", err)
	}
	wrote := 0
	for _, b := range browsers(env) {
		data, err := ManifestFor(b.Family, program)
		if err != nil {
			return err
		}
		if env.GOOS == "windows" {
			file := filepath.Join(dir, nativehost.HostName+"."+b.Family+".json")
			if err := writeFile(file, data, 0o600); err != nil {
				return err
			}
			if env.Registry == nil {
				return errors.New("no registry")
			}
			if err := env.Registry.Set(b.Key, file); err != nil {
				return fmt.Errorf("registry %s: %w", b.Key, err)
			}
			fmt.Fprintf(env.Out, "%s: HKCU\\%s -> %s\n", b.Name, b.Key, file)
			wrote++
			continue
		}
		if _, err := os.Stat(b.Root); err != nil && !all {
			fmt.Fprintf(env.Out, "%s: not used on this computer (nothing written; --all writes it anyway)\n", b.Name)
			continue
		}
		file := filepath.Join(b.Dir, nativehost.HostName+".json")
		if err := writeFile(file, data, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(env.Out, "%s: %s\n", b.Name, file)
		wrote++
	}
	fmt.Fprintf(env.Out, "The program is in %s\n", program)
	if wrote == 0 {
		fmt.Fprintln(env.Out, "No browser was found: open Chrome, Edge or Firefox once and run this again, or use --all.")
	}
	if env.GOOS == "darwin" {
		fmt.Fprintf(env.Out, "If the browser cannot start it (a file that was downloaded is quarantined): xattr -d com.apple.quarantine %q\n", program)
	}
	return nil
}

// Uninstall removes what Install wrote, and only that: the manifests (the files and the keys), the copy of the program, and the folder when
// it is empty.
func Uninstall(env Env) error {
	dir := ProgramDir(env)
	for _, b := range browsers(env) {
		if env.GOOS == "windows" {
			if env.Registry != nil {
				if _, ok := env.Registry.Get(b.Key); ok {
					if err := env.Registry.Delete(b.Key); err != nil {
						return err
					}
					fmt.Fprintf(env.Out, "removed HKCU\\%s\n", b.Key)
				}
			}
			continue
		}
		file := filepath.Join(b.Dir, nativehost.HostName+".json")
		if err := os.Remove(file); err == nil {
			fmt.Fprintf(env.Out, "removed %s\n", file)
		}
	}
	for _, name := range []string{binaryName(env.GOOS), nativehost.HostName + ".chromium.json", nativehost.HostName + ".firefox.json"} {
		if err := os.Remove(filepath.Join(dir, name)); err == nil {
			fmt.Fprintf(env.Out, "removed %s\n", filepath.Join(dir, name))
		}
	}
	if err := os.Remove(dir); err == nil {
		fmt.Fprintf(env.Out, "removed %s\n", dir)
	}
	return nil
}

// Doctor says what is where, and what does not work, without changing anything.
func Doctor(env Env) error {
	dir := ProgramDir(env)
	program := filepath.Join(dir, binaryName(env.GOOS))
	_, perr := os.Stat(program)
	fmt.Fprintf(env.Out, "program: %s (%s)\n", program, map[bool]string{true: "there", false: "not there: run install-native-host"}[perr == nil])
	problems := 0
	names := []string{}
	lines := map[string]string{}
	for _, b := range browsers(env) {
		var where, pointsAt string
		if env.GOOS == "windows" {
			where = `HKCU\` + b.Key
			if env.Registry != nil {
				if v, ok := env.Registry.Get(b.Key); ok {
					pointsAt = manifestPath(v)
				}
			}
		} else {
			where = filepath.Join(b.Dir, nativehost.HostName+".json")
			pointsAt = manifestPath(where)
		}
		state := "no manifest"
		if pointsAt != "" {
			if _, err := os.Stat(pointsAt); err != nil {
				state = "points at " + pointsAt + ", which is not there"
				problems++
			} else {
				state = "ok -> " + pointsAt
			}
		}
		names = append(names, b.Name)
		lines[b.Name] = fmt.Sprintf("%-9s %s: %s\n", b.Name, where, state)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprint(env.Out, lines[n])
	}
	for _, p := range containerBrowsers(env) {
		fmt.Fprintf(env.Out, "warning: %s: a browser in a snap or flatpak cannot start a program of the user; it is not supported\n", p)
		problems++
	}
	if env.GOOS == "darwin" {
		fmt.Fprintf(env.Out, "macOS: if the browser cannot start the program, clear the quarantine: xattr -d com.apple.quarantine %q\n", program)
	}
	if problems > 0 {
		return fmt.Errorf("%d problem(s)", problems)
	}
	return nil
}

// manifestPath reads the "path" of a manifest file ("" when there is none or it cannot be read)
func manifestPath(file string) string {
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	var m Manifest
	if json.Unmarshal(data, &m) != nil {
		return ""
	}
	return m.Path
}

// containerBrowsers lists the browsers that live in a snap or a flatpak, which the host cannot be started from
func containerBrowsers(env Env) []string {
	if env.GOOS != "linux" {
		return nil
	}
	var found []string
	for _, p := range []string{
		filepath.Join(env.Home, "snap", "firefox"), filepath.Join(env.Home, "snap", "chromium"),
		filepath.Join(env.Home, ".var", "app", "org.mozilla.firefox"), filepath.Join(env.Home, ".var", "app", "com.google.Chrome"),
		filepath.Join(env.Home, ".var", "app", "org.chromium.Chromium"), filepath.Join(env.Home, ".var", "app", "com.microsoft.Edge"),
	} {
		if _, err := os.Stat(p); err == nil {
			found = append(found, p)
		}
	}
	return found
}
