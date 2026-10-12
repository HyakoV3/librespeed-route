package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestVersionsThatABrowserTakes(t *testing.T) {
	for _, v := range []string{"0.2.1", "1", "1.2", "1.2.3.4", "0.0.0", "65535.65535.65535.65535", "10.20.30"} {
		if !versionOK(v) {
			t.Errorf("%q was refused", v)
		}
	}
	for _, v := range []string{"", "0.2.1-dev", "v0.2.1", "1.2.3.4.5", "1..2", ".1", "1.", "65536", "1.2.3-rc1", "01.2", "a.b", "1.2.3+x", " 1.2", "99999999"} {
		if versionOK(v) {
			t.Errorf("%q was taken", v)
		}
	}
}

// The extension that is built is one that Chrome loads (a version of numbers) and that the program lets in (the key of development gives
// the id): the manifests are read back from the folders and from the zips.
func TestBuild(t *testing.T) {
	out := t.TempDir()
	if err := build(filepath.Join("..", "..", "extension"), out, "0.2.1-dev", true, true); err == nil {
		t.Fatal("a version that Chrome refuses was built")
	}
	if err := build(filepath.Join("..", "..", "extension"), out, "0.2.1", true, true); err != nil {
		t.Fatal(err)
	}
	// The zip of a development build says so in its name, and has the key; the one of the store does not (below)
	sideload, err := zip.OpenReader(filepath.Join(out, "librespeed-route-extension-chromium-sideload-0.2.1.zip"))
	if err != nil {
		t.Fatal(err)
	}
	hasKey := false
	for _, f := range sideload.File {
		if f.Name == "manifest.json" {
			rc, _ := f.Open()
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(rc)
			rc.Close()
			hasKey = bytes.Contains(buf.Bytes(), []byte(`"key"`))
		}
	}
	sideload.Close()
	if !hasKey {
		t.Error("the sideload zip has no key")
	}
	for _, b := range []string{"chromium", "firefox"} {
		data, err := os.ReadFile(filepath.Join(out, b, "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		if m["version"] != "0.2.1" {
			t.Errorf("%s: version %v", b, m["version"])
		}
		_, hasKey := m["key"]
		if hasKey != (b == "chromium") {
			t.Errorf("%s: key %v in a development build", b, hasKey)
		}
	}
	// Without -dev, no key in any
	store := t.TempDir()
	if err := build(filepath.Join("..", "..", "extension"), store, "0.2.1", false, true); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(filepath.Join(store, "librespeed-route-extension-chromium-0.2.1.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	found := false
	for _, f := range zr.File {
		if f.Name != "manifest.json" {
			continue
		}
		found = true
		rc, _ := f.Open()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(rc)
		rc.Close()
		if bytes.Contains(buf.Bytes(), []byte(`"key"`)) {
			t.Error("the zip for the store has the key of development")
		}
	}
	if !found {
		t.Error("no manifest in the zip")
	}
}

// The icons of the extension are the favicon of the pages, in the four sizes that the manifests name, and they go into every build as they are
func TestIconsAreThoseOfTheFolderAndTheRightSize(t *testing.T) {
	root := filepath.Join("..", "..", "extension")
	out := t.TempDir()
	if err := build(root, out, "0.1.0", false, false); err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{16, 32, 48, 128} {
		name := fmt.Sprintf("%d.png", size)
		want, err := os.ReadFile(filepath.Join(root, "icons", name))
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(want))
		if err != nil || cfg.Width != size || cfg.Height != size {
			t.Fatalf("%s: %v %dx%d", name, err, cfg.Width, cfg.Height)
		}
		for _, b := range []string{"chromium", "firefox"} {
			got, err := os.ReadFile(filepath.Join(out, b, "icons", name))
			if err != nil || !bytes.Equal(got, want) {
				t.Errorf("%s/%s: %v", b, name, err)
			}
		}
	}
}
