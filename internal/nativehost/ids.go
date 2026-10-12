package nativehost

// HostName is the name of the native messaging host (lower case letters, digits, "_" and "."). It is under the namespace of this project,
// not of the LibreSpeed organisation.
const HostName = "io.github.hyakov3.librespeed_route"

// The extensions that may start the host. Anything else that the browser tells is refused before a byte is read.
//
// ChromeIDs: the id of the extension in Chrome, Chromium and Edge (with the key of extension/dev/public-key.txt it is the first one, for
// the extension that is loaded from the folder; the ids of the stores are added here when the items exist, in the same release as the
// manifests that name them). GeckoIDs: browser_specific_settings.gecko.id of the Firefox manifest.
var (
	ChromeIDs = []string{"gcgijalhbacnoacnklibiffbacdchjgm"}
	GeckoIDs  = []string{"librespeed-route@hyakov3.github.io"}
)
