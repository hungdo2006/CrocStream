# Known issues and limits

- Prototype only: no audio, remote control, recording, accounts or multiple viewers.
- The relay is a separate process and requires the operator to make its TCP
  port reachable to the other participant. The included default binds to
  `127.0.0.1` for same-machine demos. To listen on a LAN interface, explicitly
  pass that interface's private IPv4 address and allow the port only on a
  trusted/private network. Do not expose an unauthenticated relay to the
  public Internet.
- Each app needs the same relay address/password; the broadcaster separately
  shares its one-time invitation. Protect both secrets and use a trusted relay.
- No relay discovery, automatic reconnect, email/sign-in, secure credential
  storage, or one-click firewall configuration.
- Windows x64 only. This ZIP is portable and unsigned; no installer, signing
  certificate, SmartScreen reputation or automatic updates.
- Screen capture is GDI-based. Protected content, HDR color handling and
  display hot-unplug behavior have not been comprehensively certified.
- Actual remote-screen capture/render performance has not been tested over the
  Internet. The G4 remote proof used synthetic pixels; G5 capture stress used
  local loopback.
- A desktop change may produce low/negative delta savings. Reported FPS, RTT and
  resource metrics are measured for the current machine and interval only.
  RTT is round trip, not end-to-end latency.
- Closing either app ends the session. Stop/Leave before switching role; an
  unresponsive Windows capture worker can exceed the normal shutdown target.
- `Local preview` is a separate local-only mode and does not connect to a peer.
