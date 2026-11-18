# CrocStream 0.0.1 — Windows portable

1. Extract the ZIP to a writable folder.
2. Open `crocstream.exe`. No Go, Node, npm or Wails installation needed.
3. For sharing, both machines need the same relay address/password. See
   [DEMO.md](DEMO.md) for a same-PC or trusted private-LAN demonstration.

Repository maintainers can rerun the engineering UI acceptance with
`app/g6/Test-ProductionUI.ps1 -Directory <extracted-folder>`. It exercises the
shipped executables over loopback with Windows UI Automation and no CDP/test
build. It temporarily uses the Windows clipboard to invoke **Copy invitation**
and refuses to run unless the clipboard is empty; no invitation is printed or
saved. It sends only loopback traffic and never writes frames/screenshots.

Requires Windows x64 with Microsoft Edge WebView2 Evergreen Runtime installed.
The runtime is not bundled or installed silently. If it is missing, obtain it
from Microsoft's official WebView2 download page or your system administrator:
<https://developer.microsoft.com/microsoft-edge/webview2/>

This is an **unsigned technical prototype**, not a production conferencing
tool. Windows may show an unsigned-app/reputation warning; verify the trusted
source and checksum before running. Do not disable security controls.

## In this folder

- `crocstream.exe`: desktop Share/Watch and local preview.
- `crocstream-relay.exe`, `Start-DemoRelay.ps1`: optional manually operated demo
  relay using the pinned croc implementation; loopback/private IPv4 only.
- `Verify-Package.ps1`, `manifest.json`: allowlisted content hashes.
- `dependencies.json`, `build-info.txt`: exact linked dependency/build metadata.
- `licenses/`, `NOTICE.txt`: attribution and distribution scope.
- `KNOWN_ISSUES.md`: measured limits and deferred features.

No relay, invitation, password or screen image is bundled. The app never
captures before Start and does not record frames to disk. WebView2 can create
its normal browser-profile/cache data; portable does not mean zero OS/profile
footprint. App credentials are not intentionally persisted.

To remove, stop the session and relay, close the apps, then delete the extracted
folder. There is no installer, registry registration, autostart or updater
configured by this package. Existing WebView2 runtime/profile data is not
removed by deleting the package. Keep third-party notices with any copied
executables. This custom protocol is not stock croc file-transfer compatible.
