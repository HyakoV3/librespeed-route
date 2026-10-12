package nativehost

import (
	"regexp"
	"strings"
)

var reParentWindow = regexp.MustCompile(`^--parent-window=[0-9]{1,20}$`)

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// LooksLikeABrowserCall says whether the arguments are the ones that a browser gives, whoever it is: the program then does not read its
// options, it is the host (and it ends at once if the caller is not one of ours). Chrome, Chromium and Edge give the origin of the extension
// ("chrome-extension://<id>/"), and on Windows a "--parent-window=<n>" after it; Firefox gives the path of the manifest and the id.
func LooksLikeABrowserCall(args []string, geckoIDs []string) bool {
	if len(args) >= 1 && strings.HasPrefix(args[0], "chrome-extension://") {
		return true
	}
	return len(args) == 2 && strings.HasSuffix(args[0], ".json") && contains(geckoIDs, args[1])
}

// CheckCaller says whether the arguments are those of one of our extensions, exactly: nothing more, nothing less.
func CheckCaller(args []string, goos string, chromeIDs, geckoIDs []string) bool {
	if len(args) >= 1 && strings.HasPrefix(args[0], "chrome-extension://") {
		const prefix = "chrome-extension://"
		rest := args[0][len(prefix):]
		if !strings.HasSuffix(rest, "/") || !contains(chromeIDs, strings.TrimSuffix(rest, "/")) {
			return false
		}
		switch len(args) {
		case 1:
			return true
		case 2:
			return goos == "windows" && reParentWindow.MatchString(args[1])
		}
		return false
	}
	if len(args) == 2 {
		return absolute(args[0], goos) && baseName(args[0]) == HostName+".json" && contains(geckoIDs, args[1])
	}
	return false
}

var reDrive = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

// absolute and baseName follow the system that the program was built for, not the one that runs the test
func absolute(path, goos string) bool {
	if goos == "windows" {
		return reDrive.MatchString(path)
	}
	return strings.HasPrefix(path, "/")
}

func baseName(path string) string {
	if i := strings.LastIndexAny(path, "/\\"); i >= 0 {
		return path[i+1:]
	}
	return path
}
