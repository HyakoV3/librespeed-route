# Manual test of the extension (before a release)

The automated tests (`cd extension && npx playwright test`) run the Chromium of Playwright. What they cannot see is checked by hand, in
Chrome, Edge and Firefox (the minimum versions of the manifests), on Windows, macOS and Linux when there is one at hand.

1. **Clean install:** load the extension, without the program. Open the popup on an https page: it says that the program is not installed,
   shows the command, and **Details** (closed) has what the browser said. **Help** and **About** are closed and open with Enter/Space.
2. **With the program:** `librespeed-route install-native-host`, open the popup again: it says it can trace to the host of the tab.
3. **Trace:** press Trace. The icon counts the seconds; the popup says Done; **Copy** says "Copied" and the clipboard has the line.
   (Firefox: the clipboard works in the popup; if it does not, the line is selected and the note says to press Ctrl+C.)
4. **Page that cannot be traced:** a tab of the browser (settings, new tab) and an `http://` page: the popup says to open an https page.
5. **Site:** Allow this site: the prompt names the exact host; after it, the note says the page can start a trace; **Turn off** says it is
   off for that host and the button of the page goes off. In Firefox the popup closes itself after asking.
6. **Update:** a program of another version: the popup says to update both.
7. **About:** the version is the one of the installed extension; the three links open the project on a click, in a new tab.
8. **Looks:** light and dark theme, 200 % zoom, the longest language (Swedish, Portuguese), keyboard only (Tab order, visible focus),
   a screen reader reading the message when it changes.
