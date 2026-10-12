package install

import (
	"golang.org/x/sys/windows/registry"
)

// SystemRegistry is the registry of the user.
type SystemRegistry struct{}

func (SystemRegistry) Set(key, value string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, key, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue("", value)
}

func (SystemRegistry) Get(key string) (string, bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER, key, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer k.Close()
	v, _, err := k.GetStringValue("")
	return v, err == nil
}

func (SystemRegistry) Delete(key string) error {
	return registry.DeleteKey(registry.CURRENT_USER, key)
}
