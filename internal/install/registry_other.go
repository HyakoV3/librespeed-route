//go:build !windows

package install

// SystemRegistry does not exist outside Windows: the manifests are files.
type SystemRegistry struct{}

func (SystemRegistry) Set(string, string) error  { return nil }
func (SystemRegistry) Get(string) (string, bool) { return "", false }
func (SystemRegistry) Delete(string) error       { return nil }
