package i18n

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// SystemLocale is the language of the user on Windows ("pt-BR"), from the system (the environment variables of Windows do not have it).
func SystemLocale() string {
	var buf [85]uint16 // LOCALE_NAME_MAX_LENGTH
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")
	n, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}
