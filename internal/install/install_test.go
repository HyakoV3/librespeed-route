package install

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyakoV3/librespeed-route/internal/nativehost"
)

type fakeRegistry map[string]string

func (r fakeRegistry) Set(key, value string) error { r[key] = value; return nil }
func (r fakeRegistry) Get(key string) (string, bool) {
	v, ok := r[key]
	return v, ok
}
func (r fakeRegistry) Delete(key string) error { delete(r, key); return nil }

func setup(t *testing.T, goos string) (Env, *bytes.Buffer) {
	t.Helper()
	home := t.TempDir()
	exe := filepath.Join(t.TempDir(), "librespeed-route-source")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\necho the program\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	env := Env{GOOS: goos, Home: home, Exe: exe, Out: &out, Registry: fakeRegistry{}}
	if goos == "windows" {
		env.LocalAppData = filepath.Join(home, "AppData", "Local")
	}
	return env, &out
}

func mkdir(t *testing.T, path ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(path...), 0o755); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

const wantChromium = `{
  "name": "io.github.hyakov3.librespeed_route",
  "description": "librespeed-route: traces the route from this computer to the server of the page that you clicked on, for the route page of LibreSpeed",
  "path": "%s",
  "type": "stdio",
  "allowed_origins": [
    "chrome-extension://gcgijalhbacnoacnklibiffbacdchjgm/"
  ]
}
`

const wantFirefox = `{
  "name": "io.github.hyakov3.librespeed_route",
  "description": "librespeed-route: traces the route from this computer to the server of the page that you clicked on, for the route page of LibreSpeed",
  "path": "%s",
  "type": "stdio",
  "allowed_extensions": [
    "librespeed-route@hyakov3.github.io"
  ]
}
`

func TestInstallOnLinuxWritesTheBrowsersThatAreUsed(t *testing.T) {
	env, out := setup(t, "linux")
	mkdir(t, env.Home, ".config", "google-chrome")
	mkdir(t, env.Home, ".mozilla")
	// A manifest of another host in the same folder is not touched
	other := filepath.Join(env.Home, ".config", "google-chrome", "NativeMessagingHosts", "other.host.json")
	mkdir(t, filepath.Dir(other))
	if err := os.WriteFile(other, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Install(env, false); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(env.Home, ".local", "share", "librespeed-route", "librespeed-route")
	data, err := os.ReadFile(program)
	if err != nil || string(data) != "#!/bin/sh\necho the program\n" {
		t.Fatalf("the program was not copied: %v %q", err, data)
	}
	if info, _ := os.Stat(program); info.Mode().Perm() != 0o755 {
		t.Errorf("mode %v", info.Mode())
	}
	chrome, _ := os.ReadFile(filepath.Join(env.Home, ".config", "google-chrome", "NativeMessagingHosts", nativehost.HostName+".json"))
	if string(chrome) != strings.Replace(wantChromium, "%s", program, 1) {
		t.Errorf("chrome manifest:\n%s", chrome)
	}
	firefox, _ := os.ReadFile(filepath.Join(env.Home, ".mozilla", "native-messaging-hosts", nativehost.HostName+".json"))
	if string(firefox) != strings.Replace(wantFirefox, "%s", program, 1) {
		t.Errorf("firefox manifest:\n%s", firefox)
	}
	// Edge and Chromium were not used: nothing for them
	for _, p := range []string{".config/microsoft-edge", ".config/chromium"} {
		if exists(filepath.Join(env.Home, p)) {
			t.Errorf("%s was made", p)
		}
	}
	if !strings.Contains(out.String(), "Edge: not used") {
		t.Errorf("output: %s", out)
	}
	// Twice is the same
	if err := Install(env, false); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(filepath.Join(env.Home, ".config", "google-chrome", "NativeMessagingHosts", nativehost.HostName+".json"))
	if !bytes.Equal(again, chrome) {
		t.Error("the second install changed the manifest")
	}
	// Uninstall takes away what was written and only that
	out.Reset()
	if err := Uninstall(env); err != nil {
		t.Fatal(err)
	}
	if exists(program) || exists(filepath.Dir(program)) {
		t.Error("the program is still there")
	}
	for _, f := range []string{
		filepath.Join(env.Home, ".config", "google-chrome", "NativeMessagingHosts", nativehost.HostName+".json"),
		filepath.Join(env.Home, ".mozilla", "native-messaging-hosts", nativehost.HostName+".json"),
	} {
		if exists(f) {
			t.Errorf("%s is still there", f)
		}
	}
	if !exists(other) {
		t.Error("the manifest of another host was removed")
	}
}

func TestInstallAllWritesEveryBrowser(t *testing.T) {
	env, _ := setup(t, "linux")
	if err := Install(env, true); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		".config/google-chrome/NativeMessagingHosts", ".config/chromium/NativeMessagingHosts",
		".config/microsoft-edge/NativeMessagingHosts", ".mozilla/native-messaging-hosts",
	} {
		if !exists(filepath.Join(env.Home, p, nativehost.HostName+".json")) {
			t.Errorf("%s has no manifest", p)
		}
	}
	// No browser at all and no --all: it says so
	empty, out := setup(t, "linux")
	if err := Install(empty, false); err != nil || !strings.Contains(out.String(), "No browser was found") {
		t.Errorf("%v %s", err, out)
	}
}

func TestInstallOnMac(t *testing.T) {
	env, out := setup(t, "darwin")
	app := filepath.Join(env.Home, "Library", "Application Support")
	mkdir(t, app, "Google", "Chrome")
	mkdir(t, app, "Mozilla")
	if err := Install(env, false); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(app, "librespeed-route", "librespeed-route")
	for _, f := range []string{
		filepath.Join(app, "Google", "Chrome", "NativeMessagingHosts", nativehost.HostName+".json"),
		filepath.Join(app, "Mozilla", "NativeMessagingHosts", nativehost.HostName+".json"),
	} {
		data, err := os.ReadFile(f)
		if err != nil || !strings.Contains(string(data), `"path": "`+program+`"`) {
			t.Errorf("%s: %v %s", f, err, data)
		}
	}
	if !strings.Contains(out.String(), "xattr -d com.apple.quarantine") {
		t.Errorf("no word about the quarantine: %s", out)
	}
}

func TestInstallOnWindowsUsesTheRegistryOfTheUser(t *testing.T) {
	env, _ := setup(t, "windows")
	reg := env.Registry.(fakeRegistry)
	if err := Install(env, false); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(env.LocalAppData, "librespeed-route")
	want := map[string]string{
		`Software\Google\Chrome\NativeMessagingHosts\` + nativehost.HostName:  filepath.Join(dir, nativehost.HostName+".chromium.json"),
		`Software\Chromium\NativeMessagingHosts\` + nativehost.HostName:       filepath.Join(dir, nativehost.HostName+".chromium.json"),
		`Software\Microsoft\Edge\NativeMessagingHosts\` + nativehost.HostName: filepath.Join(dir, nativehost.HostName+".chromium.json"),
		`Software\Mozilla\NativeMessagingHosts\` + nativehost.HostName:        filepath.Join(dir, nativehost.HostName+".firefox.json"),
	}
	if len(reg) != len(want) {
		t.Errorf("keys: %v", reg)
	}
	for k, v := range want {
		if reg[k] != v {
			t.Errorf("%s = %q, want %q", k, reg[k], v)
		}
	}
	data, _ := os.ReadFile(filepath.Join(dir, nativehost.HostName+".chromium.json"))
	if !strings.Contains(string(data), filepath.Join(dir, "librespeed-route.exe")) || !strings.Contains(string(data), "allowed_origins") {
		t.Errorf("chromium manifest: %s", data)
	}
	if err := Uninstall(env); err != nil {
		t.Fatal(err)
	}
	if len(reg) != 0 || exists(dir) {
		t.Errorf("left: %v %v", reg, exists(dir))
	}
	// A folder that cmd.exe would read is refused
	for _, bad := range []string{`C:\Users\a&b`, `C:\Users\100%`, `C:\Users\a"b`, `C:\Users\a^b`, `C:\Users\a!b`} {
		e, _ := setup(t, "windows")
		e.LocalAppData = bad
		if err := Install(e, false); err != ErrPath {
			t.Errorf("%s: %v", bad, err)
		}
	}
}

func TestDoctor(t *testing.T) {
	env, out := setup(t, "linux")
	err := Doctor(env)
	if err != nil || !strings.Contains(out.String(), "not there: run install-native-host") || !strings.Contains(out.String(), "no manifest") {
		t.Errorf("before: %v %s", err, out)
	}
	out.Reset()
	mkdir(t, env.Home, ".config", "google-chrome")
	if err := Install(env, false); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Doctor(env); err != nil || !strings.Contains(out.String(), "Chrome") || !strings.Contains(out.String(), ": ok -> ") {
		t.Errorf("after: %v %s", err, out)
	}
	// The program was deleted, the manifest is left: a problem
	_ = os.Remove(filepath.Join(env.Home, ".local", "share", "librespeed-route", "librespeed-route"))
	out.Reset()
	if err := Doctor(env); err == nil || !strings.Contains(out.String(), "which is not there") {
		t.Errorf("a missing program: %v %s", err, out)
	}
	// A browser in a snap
	mkdir(t, env.Home, "snap", "firefox")
	out.Reset()
	if err := Doctor(env); err == nil || !strings.Contains(out.String(), "snap or flatpak") {
		t.Errorf("snap: %v %s", err, out)
	}
}

// Whatever else is in the folder of the program is not the program's: uninstall leaves it, and the folder with it
func TestUninstallLeavesWhatIsNotItsOwn(t *testing.T) {
	env, _ := setup(t, "linux")
	if err := Install(env, true); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(env.Home, ".local", "share", "librespeed-route")
	mine := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(mine, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(env); err != nil {
		t.Fatal(err)
	}
	if !exists(mine) || exists(filepath.Join(dir, "librespeed-route")) {
		t.Errorf("notes: %v, program: %v", exists(mine), exists(filepath.Join(dir, "librespeed-route")))
	}
}
