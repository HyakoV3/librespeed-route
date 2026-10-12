//go:build !windows

package cli

// ownsConsole is only for Windows: the double click of the other systems does not open a console.
func ownsConsole() bool { return false }
