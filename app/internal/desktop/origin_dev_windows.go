//go:build !production && windows

package desktop

import (
	"net/url"
	"os"
)

func TrustedOrigin() string {
	u, err := url.Parse(os.Getenv("FRONTEND_DEVSERVER_URL"))
	if err == nil && u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost") && u.Port() == "9245" {
		return "http://wails.localhost:9245"
	}
	return "http://wails.localhost"
}
