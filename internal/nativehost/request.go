package nativehost

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
)

// Protocol is the version of the messages. A request with another "v" is refused.
const Protocol = 1

// The words of the replies: a closed set, no free text.
const (
	CodeRefused     = "refused"
	CodeNoTool      = "no-tool"
	CodeFailed      = "failed"
	CodeSilent      = "silent"
	CodeBusy        = "busy"
	CodeBadRequest  = "bad-request"
	CodeInterrupted = "interrupted"
	CodeTooLong     = "too-long"
)

var errBadRequest = errors.New("bad request")

// Request is what the extension sends: one of "hello" (is the host there, which version, which tool) and "trace" (trace to the origin).
// The fields that are not known are refused, and so is a value of the wrong kind.
type Request struct {
	V      json.RawMessage `json:"v"`
	ID     string          `json:"id"`
	Type   string          `json:"type"`
	Origin string          `json:"origin,omitempty"`
	Family json.RawMessage `json:"family,omitempty"`
}

var (
	reID     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)
	reOrigin = regexp.MustCompile(`^https://([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*)(:[0-9]{1,5})?$`)
)

// ParseRequest reads one request, strictly: one JSON object and nothing after it, no field that is not known, "v" exactly 1, a
// short id, the type "hello", "trace" or "cancel", and for a trace an origin of the form https://host[:port] (letters in lower case, digits,
// "-" and ".", no user name, path, query or fragment) and a family that is exactly the number 0, 4 or 6. The host name is the part that is
// used; the port is not.
func ParseRequest(body []byte) (req Request, host string, family int, err error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return Request{}, "", 0, errBadRequest
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return Request{}, "", 0, errBadRequest
	}
	// Nothing after the object
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		return Request{}, "", 0, errBadRequest
	}
	if string(req.V) != "1" || !reID.MatchString(req.ID) {
		return Request{}, "", 0, errBadRequest
	}
	switch req.Type {
	case "hello", "cancel":
		if req.Origin != "" || req.Family != nil {
			return Request{}, "", 0, errBadRequest
		}
		return req, "", 0, nil
	case "trace":
		if len(req.Origin) > 260 {
			return Request{}, "", 0, errBadRequest
		}
		m := reOrigin.FindStringSubmatch(req.Origin)
		if m == nil {
			return Request{}, "", 0, errBadRequest
		}
		host = m[1]
		if len(host) > 253 {
			return Request{}, "", 0, errBadRequest
		}
		if port := m[5]; port != "" {
			if p := strings.TrimPrefix(port, ":"); len(p) > 5 || p == "" {
				return Request{}, "", 0, errBadRequest
			}
		}
		switch string(req.Family) {
		case "", "0":
			family = 0
		case "4":
			family = 4
		case "6":
			family = 6
		default:
			return Request{}, "", 0, errBadRequest
		}
		return req, host, family, nil
	}
	return Request{}, "", 0, errBadRequest
}
