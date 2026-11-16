# G3 dependency-notice scope

`reviewed-20261116` is the **initial generated draft**, retained for provenance.
It contains six packages reported as license-field-only. The
`final-20261116` snapshot supplements those with source/README notices and
matching project license text; `Collect-Notices.ps1` records hashes/source paths.

Shipping application runtime: Wails 3.1.0 (MIT, nested native notices retained),
go-ole 1.3.0 (MIT), x/sys 0.50.0 (BSD), coder/websocket 1.8.14 (ISC),
existing LZ4 4.1.33 (BSD), React/ReactDOM/scheduler (MIT) and Wails JS runtime
3.1.0 (MIT). Wails JS embeds additional callable/nanoid/HTMX notices; complete
source notice blocks are appended to the bundle.

The installed npm development/build/test closure is inventoried separately
from runtime redistribution: TypeScript Apache-2.0, Playwright Apache-2.0,
ESLint/Vite/Vitest and associated modules under the terms in their notices.
The runtime npm package omits a top-level LICENSE; its repository is the
same Wails project/version whose full MIT license is included from the Go
module. Dev-only `esrecurse` and `imurmurhash` carry full notices in JS/README.
`@humanfs/types` declares Apache-2.0 and shares the humanfs project license;
the native rolldown package declares MIT and shares the rolldown license.

Dev-only `natural-compare 1.4.0` ships author/copyright/MIT-reference text but
no complete MIT text. Its bundled source/README is retained as received.
Development tools/native bundler binaries are not shipped with the executable.
This engineering inventory is not the final G6 redistribution review;
G6 must check its exact packaged assets, compiler/runtime notices and any
additional nested code before distributing development tools or installers.

G0's already-reviewed embedded Go/native/race notices remain in
`../../g0/licenses/`; G2 LZ4 notice remains in `../../THIRD_PARTY_NOTICES.md`.
