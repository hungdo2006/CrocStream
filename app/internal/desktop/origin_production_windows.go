//go:build production && windows

package desktop

func TrustedOrigin() string { return "http://wails.localhost" }
