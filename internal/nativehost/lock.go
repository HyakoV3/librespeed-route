package nativehost

import (
	"os"
	"path/filepath"
	"time"
)

// Only one trace at a time, and not two within a few seconds, across the processes of the host (each connection of the browser starts a
// new one). Two files in a folder of the user: trace.lock exists while a trace runs (it is taken as left over after StaleLock), and trace.last
// has the time when the last one ended. No host name or address is written in them.
const (
	StaleLock = 150 * time.Second
	MinGap    = 10 * time.Second
)

// acquire takes the lock, or says why not ("busy"). release must be called when the trace is over.
func acquire(dir string, now time.Time) (release func(), ok bool) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, false
	}
	lock := filepath.Join(dir, "trace.lock")
	last := filepath.Join(dir, "trace.last")
	if info, err := os.Stat(last); err == nil && now.Sub(info.ModTime()) < MinGap && info.ModTime().Before(now.Add(MinGap)) {
		return nil, false
	}
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = f.Close()
			return func() {
				_ = os.Remove(lock)
				if f, err := os.OpenFile(last, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600); err == nil {
					_ = f.Close()
				}
				_ = os.Chtimes(last, time.Now(), time.Now())
			}, true
		}
		info, serr := os.Stat(lock)
		if serr != nil || now.Sub(info.ModTime()) <= StaleLock {
			return nil, false
		}
		// Left over by a process that did not end: taken away once, then tried again
		_ = os.Remove(lock)
	}
	return nil, false
}
