//go:build !g3test

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func browserOptions() application.WindowsOptions { return application.WindowsOptions{} }
