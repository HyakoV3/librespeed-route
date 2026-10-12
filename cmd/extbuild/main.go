// Command extbuild builds the browser extension from extension/: for each browser a folder (dist/<browser>/) with the files of extension/src
// as they are, the manifest of that browser with the version, and the icons; with -zip also a zip of each, the same bytes every time
// (sorted entries, fixed times), so that the zip of a tag can be checked against the source. With -dev the Chromium manifest gets the key
// of extension/dev/public-key.txt, so that the extension loaded from the folder has the id that the program lets in; the zip for the store
// never has it.
package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func main() {
	root := flag.String("root", "extension", "the folder of the extension")
	out := flag.String("out", "dist/extension", "where to build")
	version := flag.String("version", "0.0.0", "the version of the extension (the tag without the v)")
	dev := flag.Bool("dev", false, "the Chromium manifest gets the development key")
	doZip := flag.Bool("zip", false, "also write a zip of each browser")
	flag.Parse()
	if err := build(*root, *out, *version, *dev, *doZip); err != nil {
		fmt.Fprintln(os.Stderr, "extbuild:", err)
		os.Exit(1)
	}
}

type file struct {
	name string
	data []byte
}

// versionOK says whether a version is one that the browsers take for an extension: 1 to 4 numbers separated by dots, each from 0 to 65535
// ("0.2.1-dev" is not: Chrome refuses to load the extension)
func versionOK(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) < 1 || len(parts) > 4 {
		return false
	}
	for _, p := range parts {
		if p == "" || len(p) > 5 || (len(p) > 1 && p[0] == '0') {
			return false
		}
		n := 0
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
			n = n*10 + int(c-'0')
		}
		if n > 65535 {
			return false
		}
	}
	return true
}

func build(root, out, version string, dev, doZip bool) error {
	if !versionOK(version) {
		return fmt.Errorf("the version %q is not one that a browser takes for an extension (1 to 4 numbers separated by dots, each up to 65535)", version)
	}
	var src []file
	err := filepath.WalkDir(filepath.Join(root, "src"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(filepath.Join(root, "src"), path)
		src = append(src, file{filepath.ToSlash(rel), data})
		return nil
	})
	if err != nil {
		return err
	}
	for _, b := range []string{"chromium", "firefox"} {
		raw, err := os.ReadFile(filepath.Join(root, b, "manifest.json"))
		if err != nil {
			return err
		}
		var manifest map[string]any
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return err
		}
		manifest["version"] = version
		if dev && b == "chromium" {
			key, err := os.ReadFile(filepath.Join(root, "dev", "public-key.txt"))
			if err != nil {
				return err
			}
			manifest["key"] = strings.TrimSpace(string(key))
		}
		encoded, _ := json.MarshalIndent(manifest, "", "  ")
		files := append([]file{{"manifest.json", append(encoded, '\n')}}, src...)
		for _, size := range []int{16, 32, 48, 128} {
			files = append(files, file{fmt.Sprintf("icons/%d.png", size), icon(size)})
		}
		sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
		dir := filepath.Join(out, b)
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		for _, f := range files {
			path := filepath.Join(dir, filepath.FromSlash(f.name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, f.data, 0o644); err != nil {
				return err
			}
		}
		if doZip {
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)
			for _, f := range files {
				w, err := zw.CreateHeader(&zip.FileHeader{Name: f.name, Method: zip.Deflate, Modified: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)})
				if err != nil {
					return err
				}
				if _, err := w.Write(f.data); err != nil {
					return err
				}
			}
			if err := zw.Close(); err != nil {
				return err
			}
			name := filepath.Join(out, fmt.Sprintf("librespeed-route-extension-%s-%s.zip", b, version))
			if err := os.WriteFile(name, buf.Bytes(), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// icon draws a small route: two dots and a line between them, on a blue square with round corners. The same bytes every time.
func icon(size int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	blue := color.RGBA{11, 87, 208, 255}
	white := color.RGBA{255, 255, 255, 255}
	r := float64(size) / 5
	inside := func(x, y float64) bool {
		cx, cy := x-float64(size)/2+0.5, y-float64(size)/2+0.5
		half := float64(size) / 2
		ax, ay := abs(cx), abs(cy)
		if ax > half || ay > half {
			return false
		}
		if ax > half-r && ay > half-r {
			dx, dy := ax-(half-r), ay-(half-r)
			return dx*dx+dy*dy <= r*r
		}
		return true
	}
	dot := func(x, y, cx, cy, rad float64) bool {
		dx, dy := x-cx, y-cy
		return dx*dx+dy*dy <= rad*rad
	}
	s := float64(size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx, fy := float64(x), float64(y)
			if !inside(fx, fy) {
				continue
			}
			c := blue
			onLine := abs(fy-(s*0.5)) <= s*0.045 && fx >= s*0.25 && fx <= s*0.75
			if onLine || dot(fx, fy, s*0.25, s*0.5, s*0.12) || dot(fx, fy, s*0.75, s*0.5, s*0.12) {
				c = white
			}
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
