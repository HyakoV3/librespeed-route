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
	fmt.Fprint(out, env.Lang.T("win.menu"))
	line, _ := reader.ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	code := ExitOK
	switch answer {
	case "i":
		fmt.Fprintln(out)
		if err := install.Install(env, false); err != nil {
			fmt.Fprint(out, env.Lang.T("win.failed", err))
			code = ExitFailed
		} else {
			fmt.Fprint(out, env.Lang.T("win.installed"))
		}
	case "u":
		fmt.Fprintln(out)
		if err := install.Uninstall(env); err != nil {
			fmt.Fprint(out, env.Lang.T("win.failed", err))
			code = ExitFailed
		} else {
			fmt.Fprint(out, env.Lang.T("win.done"))
		}
	default:
		fmt.Fprint(out, env.Lang.T("win.nothing"))
	}
	fmt.Fprint(out, env.Lang.T("win.press"))
	_, _ = reader.ReadString('\n')
	return code
}
