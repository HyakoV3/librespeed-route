package route

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Options of Run.
type Options struct {
	// Timeout is the longest the tool may run (default 120 s, as the page says "up to two minutes").
	Timeout time.Duration
	// SilentStop kills the tool after this many hops in a row with no answer (default 5).
	SilentStop int
}

// Output is what the tool wrote, and why it was stopped when it did not end by itself.
type Output struct {
	Text    string // the bytes as the tool wrote them (the words may be in the code page of the system; they are never read)
	Stopped string // "", "silent" (too many silent hops in a row) or "timeout"
}

var (
	ErrToolFailed = errors.New("the tool failed and wrote no hop")
	// A number, one to three stars, and at most words with no digit (the message of tracert, in the language of the system)
	reSilentHop   = regexp.MustCompile(`^\s*\d{1,2}(\s+\*){1,3}[^0-9]*$`)
	reNoReplyLine = regexp.MustCompile(`^\s*\d{1,2}\??:\s+no reply\b`)
)

// SilentLine says whether a line is a hop in which nothing answered.
func SilentLine(line string) bool {
	return reSilentHop.MatchString(line) || reNoReplyLine.MatchString(line)
}

// Run starts the tool and reads what it writes, line by line, and stops it when the hops go silent or the time is over. The text of
// what it wrote up to then is returned (the silent hops at the end are cut later, by Parse). ctx stops it too (Ctrl-C): the error is then
// ctx.Err() and the text is what there was.
func Run(ctx context.Context, spec Spec, opt Options) (Output, error) {
	if opt.Timeout <= 0 {
		opt.Timeout = 120 * time.Second
	}
	if opt.SilentStop <= 0 {
		opt.SilentStop = 5
	}
	runCtx, cancel := context.WithTimeout(ctx, opt.Timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, spec.Path, spec.Args...)
	cmd.WaitDelay = 2 * time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &limitedWriter{w: &stderr, left: 4096}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Output{}, err
	}
	if err := cmd.Start(); err != nil {
		return Output{}, err
	}
	var text bytes.Buffer
	stopped := ""
	silent := 0
	reader := bufio.NewReaderSize(stdout, 8192)
	for {
		line, rerr := reader.ReadString('\n')
		if len(line) > 0 {
			if text.Len()+len(line) > maxText {
				stopped = "timeout"
				cancel()
				break
			}
			text.WriteString(line)
			if SilentLine(strings.TrimRight(line, "\r\n")) {
				silent++
				if silent >= opt.SilentStop {
					stopped = "silent"
					cancel()
					break
				}
			} else if reCommon.MatchString(line) || reTracepath.MatchString(line) {
				// A hop that answered, or a line of another kind that starts with a number: the silent hops are counted again
				silent = 0
			}
		}
		if rerr != nil {
			if rerr != io.EOF {
				stopped = "timeout"
			}
			break
		}
	}
	waitErr := cmd.Wait()
	if stopped == "" && errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		stopped = "timeout"
	}
	out := Output{Text: text.String(), Stopped: stopped}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if stopped == "" && waitErr != nil && !strings.Contains(out.Text, "ms") {
		return out, ErrToolFailed
	}
	return out, nil
}

type limitedWriter struct {
	w    io.Writer
	left int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	n := len(p)
	if l.left <= 0 {
		return n, nil
	}
	if len(p) > l.left {
		p = p[:l.left]
	}
	l.left -= len(p)
	_, _ = l.w.Write(p)
	return n, nil
}
