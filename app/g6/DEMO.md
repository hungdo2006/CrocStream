# CrocStream demo

This prototype shares one screen with one viewer. Use a trusted relay and
non-sensitive content. No audio, remote input, or recording.

## Prepare a relay

For a same-PC demonstration, open PowerShell in the extracted application
folder:

```powershell
.\Start-DemoRelay.ps1
```

It listens on `127.0.0.1:9009` by default and asks for a password (12+
characters). Enter the same password in both apps. It is not put in process
arguments or saved to disk. Keep the launcher open and press Enter to stop.

For two PCs on a trusted private LAN, explicitly select the relay PC's
private IPv4 interface:

```powershell
.\Start-DemoRelay.ps1 -ListenAddress '192.168.1.20:9009'
```

Replace that example address with the actual relay PC's private IPv4. Both
apps use this same address, including the app on the relay PC. The wrapper
refuses wildcard/public binds. If needed, the operator should allow TCP 9009
only from the other participant on the Private Windows Firewall profile.
Do not port-forward the demo relay or disable the firewall.

If PowerShell blocks scripts, stop and follow your organization's policy.
No global execution-policy change is required by CrocStream. The standalone
relay is not a Windows service and exits when its launcher input closes.

## Share

1. Extract the entire ZIP to a writable folder, not directly inside the ZIP.
2. Open `crocstream.exe` and choose **Chia sẻ màn hình**.
3. Enter relay address/password, select the display and resolution/FPS.
4. Click **Bắt đầu chia sẻ**. The preview appears, followed by
   **Đang chờ người xem** once relay preparation succeeds.
5. Use **Sao chép mã** to privately send the invitation to one viewer.

Nothing is captured before Start. Treat the invitation as a secret.
Stopping the session revokes it; a new session creates a new invitation.

## Watch

1. Open a second `crocstream.exe` (or the app on the second PC).
2. Choose **Tham gia bằng mã**.
3. Enter the same relay address/password and the invitation.
4. Click **Tham gia phiên**. The image appears only after authentication and
   validation of the first keyframe.

**Chi tiết luồng** contains measured capture/emitted/decoded FPS, baseline,
savings and packet sizes. Main rates exclude transport overhead. RTT is
round-trip, not end-to-end latency. No universal bandwidth/FPS claim.

## End the demo

Use **Rời phiên** on the viewer or **Dừng chia sẻ** on the sender. Both canvases
clear when disconnected. Return Home before changing roles. Close both app
windows, then press Enter in the relay launcher. The prototype does not
auto-reconnect.

## Check the download

From the folder containing the ZIP:

```powershell
Get-FileHash .\CrocStream-0.0.1-windows-amd64.zip -Algorithm SHA256
Get-Content .\CrocStream-0.0.1-windows-amd64.zip.sha256
.\CrocStream-0.0.1-windows-amd64\Verify-Package.ps1 `
  -Directory .\CrocStream-0.0.1-windows-amd64
```

Compare the checksum with a value obtained separately from your trusted
distributor. Adjacent hash files detect corruption, not authenticity if both
files came from an untrusted source. The app is unsigned: cancel on an
unexpected source or checksum. Never disable Defender/SmartScreen globally.

## Check capture without a relay

Choose **Xem trước trên máy này**. This mode is local-only and sends no images
to a peer. Close sensitive content before starting any capture.
