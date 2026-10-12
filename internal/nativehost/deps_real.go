//go:build !e2efake

package nativehost

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/HyakoV3/librespeed-route/internal/route"
)

// SystemDeps is the real system. Nothing in the environment changes what the host does (the only thing read from it is where a system
// keeps the folder of a user).
func SystemDeps(version string) Deps {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}
	return Deps{
		Version:    version,
		GOOS:       runtime.GOOS,
		SystemRoot: os.Getenv("SystemRoot"),
		Resolver:   net.DefaultResolver,
		Exists:     route.FileExists,
		Run:        route.Run,
		Now:        time.Now,
		Nonce: func() (string, error) {
			b := make([]byte, 8)
			if _, err := rand.Read(b); err != nil {
				return "", err
			}
			return hex.EncodeToString(b), nil
		},
		LockDir:   filepath.Join(dir, "librespeed-route"),
		ChromeIDs: ChromeIDs,
		GeckoIDs:  GeckoIDs,
	}
}
