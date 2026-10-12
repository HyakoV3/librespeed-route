package install

import "testing"

// The real registry of the user (only on Windows): a key of a test, written, read back and deleted
func TestTheRealRegistry(t *testing.T) {
	key := `Software\librespeed-route-test\NativeMessagingHosts\x`
	reg := SystemRegistry{}
	t.Cleanup(func() { _ = reg.Delete(key) })
	if err := reg.Set(key, `C:\x\y.json`); err != nil {
		t.Fatal(err)
	}
	if v, ok := reg.Get(key); !ok || v != `C:\x\y.json` {
		t.Fatalf("%q %v", v, ok)
	}
	if err := reg.Delete(key); err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Get(key); ok {
		t.Error("the key is still there")
	}
}
