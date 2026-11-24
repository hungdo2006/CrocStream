//go:build g3test

package desktop

import (
	"os"
	"strconv"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Test-only CDP; ordinary builds expose no debugging socket.
func BrowserOptions() application.WindowsOptions {
	port, err := strconv.Atoi(os.Getenv("CROCSTREAM_TEST_CDP_PORT"))
	if err != nil || port < 1024 || port > 65535 {
		return application.WindowsOptions{}
	}
	return application.WindowsOptions{WebviewUserDataPath: os.Getenv("CROCSTREAM_TEST_WEBVIEW_PATH"), AdditionalBrowserArgs: []string{
		"--remote-debugging-port=" + strconv.Itoa(port), "--remote-debugging-address=127.0.0.1",
	}}
}
