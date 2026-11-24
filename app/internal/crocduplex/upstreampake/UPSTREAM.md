# Derived PAKE schedule

Derived from `croc/src/pakekey/pakekey.go` at
`552f40fd2d7593316118370bec0a93478d48e715`, MIT (LICENSE alongside this file).
This is modified CrocStream experimental code, not the unmodified upstream API.

Changes: fixed purpose `peer-crocstream-g0-v1` and curve `p256`; removed caller
selection of other purposes/curves; context no longer carries those constants;
the same length-prefixed identities/transcript, HKDF 96-byte layout and
role-specific HMAC confirmation formula are retained. Crypto primitives are
imported from pinned upstream dependencies.

Review focus: verify isolation against all three upstream purposes, wrong
passwords, room and transcript mismatch. No stock croc compatibility is claimed.
