# Upstream croc reference

`croc/` contains the tracked source tree exported from the local upstream checkout:

- Project: [schollz/croc](https://github.com/schollz/croc)
- Commit: `552f40fd2d7593316118370bec0a93478d48e715`
- Imported: 2026-11-15
- License: [MIT, preserved verbatim](croc/LICENSE)
- Third-party notices: [preserved upstream notices](croc/THIRD_PARTY_NOTICES.md)

This is a source snapshot, not a submodule. Upstream `.git` metadata and local
untracked files are excluded. Tracked files, including upstream documentation,
tests and build configuration, retain their original contents. The separate
local checkout at `../croc/` remains untouched.

The snapshot is reference code for the CrocStream feasibility work. It is not
an implementation of the proposed screen-streaming application. No build or
runtime feasibility is asserted by importing it. Its license does not assign
a license to the independently authored CrocStream documents.

In the CrocStream design documents, `croc/` paths are relative to the repository
root. Keep this snapshot pristine; record any future upstream update with its
exact revision, licensing review and applicable verification.
