# Portable toolchain and environment scripts

These scripts manage the reproducible Windows build/test toolchain for CrocStream without modifying system-wide Go installations or `croc/`.

## Portable tools

Run from `D:\CrocStream` using PowerShell:

```powershell
powershell -NoProfile -File .\scripts\toolchain\Install-Toolchain.ps1 -Component Go
powershell -NoProfile -File .\scripts\toolchain\Install-Toolchain.ps1 -Component Wails
. .\scripts\toolchain\Use-Toolchain.ps1
go version
wails3 doctor
```

Tools, caches and binaries go under `$env:LOCALAPPDATA\Temp\opencode\crocstream-g0`, never into `croc/`. Go is pinned to 1.27.4; the official Windows amd64 ZIP's size and SHA-256 are checked before extraction. Wails CLI is installed as `github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0` with Go checksum verification.

`Use-Toolchain.ps1` must be **dot-sourced** so settings apply to the current PowerShell process.

## Upstream provenance inventory

```powershell
powershell -NoProfile -File .\scripts\toolchain\Inspect-Environment.ps1 -VerifyUpstream
```

Checks the committed `croc/` tree (`0e401898d47eaef76e62263483943e3aa789ad3a`), working-tree cleanliness, tool locations and optional GitHub commit metadata.

## Related tools

- License & notice scripts: [`../notices/`](../notices/)
- Specialized QA & link probes: [`../qa-probes/README.md`](../qa-probes/README.md)
- Historical G0 reports: [`../../docs/archive-gates/README.md`](../../docs/archive-gates/README.md)
