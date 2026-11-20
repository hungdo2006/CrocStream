# G0 third-party notice scope

The generated inventory covers the **compiled Windows package closures** of
`app/g0/poc` and `app/g0/buildprobe`, ordinary and race builds. It is not
`go list -m all`, not all dependencies of the Wails development CLI, and not
the future frontend/installer dependency set.

Generation (portable toolchain loaded):

```powershell
.\app\g0\Audit-Licenses.ps1 -OutputDirectory 'C:\new\empty-output-directory'
```

The destination must not exist; the tool refuses overwrites. It reads selected
module/source/embedded-asset metadata, retains module and nested license files,
inline notices, upstream derivative provenance and extra embedded-runtime
attributions. It downloads **license texts only**, not packages or executables.
Source URLs and SHA-256 values are recorded; some attribution-source URLs point
to upstream branches, so retain the reviewed generated snapshot rather than
assuming a future download has identical text.

The generated status is deliberately **review required**, never automatic
legal approval. The reviewed G0 snapshot and decision are recorded in
[G0_CLOSEOUT](../../../docs/gates/reports/G0_CLOSEOUT.md).

Authoritative reviewed snapshot: [`reviewed-20261116/inventory.json`](reviewed-20261116/inventory.json)
and [`THIRD_PARTY_NOTICES.txt`](reviewed-20261116/THIRD_PARTY_NOTICES.txt).
It covers 21 modules, 52 notice/source files, 135 inline notices and 21 embedded
assets. `generated-20261116` is an earlier draft retained for audit history,
not the authoritative bundle.

Regression check:

```powershell
.\app\g0\Test-LicenseInventory.ps1 -Directory .\app\g0\licenses\reviewed-20261116
```

## Known obligations to preserve with G0 binaries

- MIT/BSD/ISC/zlib code: retain copyright/license/disclaimer texts and applicable
  non-endorsement terms. No claim that every dependency is MIT.
- HighwayHash Apache-2.0: include full license and applicable notices; preserve
  modification notices for any future modified sources.
- EFF Short Wordlist #1: CC-BY-4.0, credit Electronic Frontier Foundation,
  include source/license links and indicate upstream removed the dice labels.
  The word list is data under its own terms, not relabeled as croc MIT.
- Modified upstream PAKE schedule: retain existing MIT notice and modification/
  source record in `app/g0/poc/internal/upstreampake/`.
- Wails nested sources: WebView loader ISC, file dialog MIT, go-ole MIT,
  Winc MIT, W32/clipboard/Chromium BSD notices.
- Embedded runtime: nanoid attribution from Wails' own modified source,
  OXC/Babel helper and generated Rolldown runtime licenses retained conservatively.
- Go standard library/vendor notices and race-runtime LLVM provenance/license
  are retained, including licenses conservatively discovered under unused
  platform stubs. An inventory entry does not mean every byte survives linking.

No WebView2 runtime, Windows system DLLs, Node, Go compiler or Wails CLI
installer is redistributed by this repository's G0 binaries. Those products
have separate installation/redistribution terms if future packaging includes
them. Wails artwork/trademarks must not be used to imply endorsement.

This is engineering dependency/notice review, not legal advice or a final
shipping clearance. Re-run and review at G6 after production packages,
React/TypeScript/assets/capture implementation and installer contents exist.
