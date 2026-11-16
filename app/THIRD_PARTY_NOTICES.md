# Application dependency notices

G3 adds owner-approved Wails 3.1.0, x/sys 0.50.0, coder/websocket 1.8.14
and locked frontend/build/test dependencies. See
[G3 notice scope and full bundle](g3/licenses/README.md). Runtime and
development tools are distinguished; G6 must audit the exact distributed
package. The LZ4 notice below remains applicable to G2 components.

G2 dependency approval: owner explicitly approved `github.com/pierrec/lz4/v4`
**v4.1.33** on 2026-11-16. Module and content hashes are pinned in `go.mod` /
`go.sum`. Source tag:
<https://github.com/pierrec/lz4/tree/v4.1.33>,
commit `b3ae24d689a8c0020f1567fc424c53bde9dfc4f7`.

- Module content: `h1:GjG1TJ1V4IzKP8L96muuuDNpTwd7D+l2ccXrjAbe014=`
- go.mod: `h1:7SE9MC2STkNtL4PIwGhjmyVwvILaGI9/COYQNBhKM/c=`
- Upstream go.mod declares Go 1.22 and no external module requirements.
- Reviewed module LICENSE, reader/writer options, frame/header handling,
  and source notice search. The downloaded module contains one LICENSE file;
  no additional inline copyright/license notices were found in Go/assembly.
- Preserve the complete notice below with redistributed application binaries.
  The separate G0 notices remain in `g0/licenses/`.

## github.com/pierrec/lz4/v4 — BSD-3-Clause

Verbatim upstream `LICENSE`:

```text
Copyright (c) 2015, Pierre Curto
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

* Redistributions of source code must retain the above copyright notice, this
  list of conditions and the following disclaimer.

* Redistributions in binary form must reproduce the above copyright notice,
  this list of conditions and the following disclaimer in the documentation
  and/or other materials provided with the distribution.

* Neither the name of xxHash nor the names of its
  contributors may be used to endorse or promote products derived from
  this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```
