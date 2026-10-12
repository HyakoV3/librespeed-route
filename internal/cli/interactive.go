package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/HyakoV3/librespeed-route/internal/install"
)

// interactive is what the program does when it is opened with a double click on Windows (no arguments, and the console is its own): a window
// that says what it is and asks, and does not close until a key is pressed. It reads from in only here; nothing else of the program reads
// its input, and the host (started by a browser) never gets here.
func interactive(in io.Reader, out io.Writer, env install.Env) int {
	reader := bufio.NewReader(in)
	fmt.Fprint(out, `librespeed-route

This program connects the browser extension "Route back for LibreSpeed pages" (Chrome, Edge, Firefox) to this computer: when you click the
extension on the page of a server, it runs the tracert of Windows and hands the route to the page, without the addresses of your own network.

  [I] Install it for this user (it is copied to %LOCALAPPDATA%\librespeed-route and registered for the browsers; no administrator rights)
  [U] Uninstall it (takes away only what the install wrote)
  [Q] Quit

To trace a route from a terminal instead:  librespeed-route.exe HOST   (or --help)

Choose [I/U/Q]: `)
	line, _ := reader.ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	code := ExitOK
	switch answer {
	case "i":
		fmt.Fprintln(out)
		if err := install.Install(env, false); err != nil {
			fmt.Fprintf(out, "\nIt did not work: %v\n", err)
			code = ExitFailed
		} else {
			fmt.Fprintln(out, "\nDone. Now add the extension to the browser, and click its icon on the page of a server.")
		}
	case "u":
		fmt.Fprintln(out)
		if err := install.Uninstall(env); err != nil {
			fmt.Fprintf(out, "\nIt did not work: %v\n", err)
			code = ExitFailed
		} else {
			fmt.Fprintln(out, "\nDone.")
		}
	default:
		fmt.Fprintln(out, "\nNothing was changed.")
	}
	fmt.Fprint(out, "\nPress Enter to close. ")
	_, _ = reader.ReadString('\n')
	return code
}
