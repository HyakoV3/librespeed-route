// Package nativehost is the native messaging host of the browser extension: the browser starts the program with its own arguments, sends
// one request as a frame (4 bytes of length in the byte order of the machine, then JSON), and the program answers with one frame of a closed
// set and ends. It never writes anything else to its output, nothing at all to its error output, and never gives the browser an address of
// the internal network of the person (only the text of LSR1, which has them left out).
package nativehost

import (
	"encoding/binary"
	"errors"
	"io"
)

// Limits: what the browser may send us (it allows much more: the host decides) and what we send it (it allows 1 MiB).
const (
	MaxRequest = 4096
	MaxReply   = 4096
)

var (
	ErrFrameSize = errors.New("the size of the frame is not one that this host takes")
)

// ReadFrame reads one frame, and refuses a size of 0 or over max before it allocates anything.
func ReadFrame(r io.Reader, max int) ([]byte, error) {
	var head [4]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return nil, err
	}
	n := binary.NativeEndian.Uint32(head[:])
	if n == 0 || n > uint32(max) {
		return nil, ErrFrameSize
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

// WriteFrame writes one frame in one call, so that a reader never sees half of it.
func WriteFrame(w io.Writer, body []byte) error {
	if len(body) == 0 || len(body) > MaxReply {
		return ErrFrameSize
	}
	out := make([]byte, 4+len(body))
	binary.NativeEndian.PutUint32(out, uint32(len(body)))
	copy(out[4:], body)
	_, err := w.Write(out)
	return err
}
