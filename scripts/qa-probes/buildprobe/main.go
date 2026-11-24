// G0 native/link probe: no desktop window, network or screen capture is created.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/schollz/croc/v11/src/comm"
	"github.com/schollz/croc/v11/src/tcp"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type statusService struct{}

func (*statusService) Status() string { return "build-probe-only" }

// Refer to real upstream signatures without opening a network connection.
var relayConnect func(context.Context, string, string, string, ...time.Duration) (*comm.Comm, string, string, string, error) = tcp.ConnectToTCPServerControlContext

func main() {
	options := application.Options{
		Name:     "CrocStream G0 build probe",
		Services: []application.Service{application.NewService(&statusService{})},
	}
	if relayConnect == nil || len(options.Services) != 1 {
		panic("build probe wiring failed")
	}
	if err := verifyNativeCapture(); err != nil {
		panic(err)
	}
	fmt.Println("G0_BUILD_PROBE_OK: linked Go/Wails/croc/Win32-GDI; synthetic bitmap copy verified; no screen captured")
}
