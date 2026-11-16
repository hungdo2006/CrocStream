package frontend

import "embed"

// dist-bootstrap.txt keeps package loading/binding generation valid on a fresh
// checkout before Vite creates dist/. Only dist/ contains index.html.
//
//go:embed all:dist*
var Assets embed.FS
