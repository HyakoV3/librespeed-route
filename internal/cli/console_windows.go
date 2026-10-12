package cli

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// ownsConsole says whether the console of the program is its own, which is what a double click on the file does (a terminal that the
// person opened has the shell in it as well).
func ownsConsole() bool {
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleProcessList")
	var ids [4]uint32
	n, _, _ := proc.Call(uintptr(unsafe.Pointer(&ids[0])), uintptr(len(ids)))
	return n == 1
}
