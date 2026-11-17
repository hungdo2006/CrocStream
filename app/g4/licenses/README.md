# G4 dependency/notice follow-up

The owner approved reuse of the pinned croc snapshot, G0 duplex/PAKE adapter,
`pake/v3 v3.2.1`, `x/crypto v0.59.0` and their transitive module closure.
The application graph uses local replacements for G0 and pristine `croc/`;
it does not fetch the upstream v11.6.5 tag as a substitute for the pinned tree.

Go's minimum-version selection also raises `coder/websocket` from 1.8.14 to
**1.8.15**, and the graph's non-Windows `godbus/dbus/v5` from 5.2.2 to **5.2.4**,
as required by the existing croc dependency graph. The app's `go.mod`/`go.sum`
pin the resolved versions. WebSocket 1.8.15 retains the ISC license. No frontend
dependency version changed in G4.

Generated snapshot: [`generated-20261117/inventory.json`](generated-20261117/inventory.json)
and [`THIRD_PARTY_NOTICES.txt`](generated-20261117/THIRD_PARTY_NOTICES.txt).
Collected with the existing `app/g3/Collect-Notices.ps1`, which refuses overwrite:

```powershell
powershell -NoProfile -File g3/Collect-Notices.ps1 -OutputDirectory g4/licenses/generated-20261117
```

Observed: **178 records**: 31 Go notice files across 24 module roots (including
the local derivative's MIT notice), 136 npm notice records, 8 supplements and
3 Wails JS inline-notice sources. Runtime source terms remain the MIT/BSD/ISC/
Apache/zlib families already reviewed at G0–G3. This collection does not mean
every module in Wails' much larger build-tool graph is linked into the app.
The generated snapshot is an inventory, not an independent legal/security audit.

- Bundle SHA-256:
  `9276C527A116A75B4CCCEFFBB4C0237CF33C23D2F1718198E58D0A7764D03585`
- Inventory SHA-256:
  `326EDBED4F04546E0B7822B9FE73A04BC9D58DF273C50519AAADEF7F10C0ED02`

Keep G0's reviewed nested/source/embedded attribution bundle in
`../../g0/licenses/`, G2's LZ4 notice in `../../THIRD_PARTY_NOTICES.md`,
and G3's npm/native/inline scope in `../../g3/licenses/README.md`. The derived
PAKE schedule's MIT/source/modification notice stays alongside its source in
`../../g0/poc/internal/upstreampake/`.

G6 must review the exact distributed binary/assets/installer, compiler/runtime
notices and any bundled development tools again before a release. Existing
development-only missing-license-text cases remain documented in G3; they are
not silently relabeled complete by this snapshot.
