module crocstream.local/g0/buildprobe

go 1.27.3

require (
	github.com/schollz/croc/v11 v11.6.5
	github.com/wailsapp/wails/v3 v3.1.0
	golang.org/x/sys v0.50.0
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coder/websocket v1.8.15 // indirect
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.4 // indirect
	github.com/kalafut/imohash v1.1.1 // indirect
	github.com/magisterquis/connectproxy v0.0.0-20200725203833-3582e84f0c9b // indirect
	github.com/mattn/go-colorable v0.1.16 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/minio/highwayhash v1.0.4 // indirect
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/schollz/pake/v3 v3.2.1 // indirect
	github.com/schollz/progressbar/v3 v3.19.1 // indirect
	github.com/tscholl2/siec v0.0.0-20240310163802-c2c6f6198406 // indirect
	github.com/twmb/murmur3 v1.2.2 // indirect
	golang.org/x/crypto v0.59.0 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/term v0.48.0 // indirect
	golang.org/x/text v0.44.0 // indirect
)

// The local snapshot tree, not the v11.6.5 tag, is the source under test.
// Verify its provenance with Inspect-Environment.ps1 before building.
replace github.com/schollz/croc/v11 => ../../../croc
