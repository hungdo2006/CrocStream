//go:build !g3test

package desktop

import "github.com/wailsapp/wails/v3/pkg/application"

func BrowserOptions() application.WindowsOptions { return application.WindowsOptions{} }
