//go:build production && windows

package main

func trustedOrigin() string { return "http://wails.localhost" }
