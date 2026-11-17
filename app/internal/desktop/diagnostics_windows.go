package desktop

import (
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Numeric process measurements only; no images, tokens or endpoint addresses.
type Diagnostics struct {
	HeapBytes       uint64  `json:"heapBytes"`
	AllocatedBytes  uint64  `json:"allocatedBytes"`
	Allocations     uint64  `json:"allocations"`
	Goroutines      int     `json:"goroutines"`
	GDIObjects      uint32  `json:"gdiObjects"`
	Handles         uint32  `json:"handles"`
	WorkingSetBytes uint64  `json:"workingSetBytes"`
	PrivateBytes    uint64  `json:"privateBytes"`
	ProcessMeasured bool    `json:"processMeasured"`
	CPUMs           float64 `json:"cpuMs"`
	CPUMeasured     bool    `json:"cpuMeasured"`
	LogicalCPUs     int     `json:"logicalCPUs"`
}

var (
	guiResources = windows.NewLazySystemDLL("user32.dll").NewProc("GetGuiResources")
	memoryInfo   = windows.NewLazySystemDLL("psapi.dll").NewProc("GetProcessMemoryInfo")
	handleCount  = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessHandleCount")
)

func (*Service) GetDiagnostics() Diagnostics {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	out := Diagnostics{HeapBytes: mem.HeapAlloc, AllocatedBytes: mem.TotalAlloc, Allocations: mem.Mallocs, Goroutines: runtime.NumGoroutine()}
	process := windows.CurrentProcess()
	gdi, _, _ := guiResources.Call(uintptr(process), 0)
	out.GDIObjects = uint32(gdi)
	ok1, _, _ := handleCount.Call(uintptr(process), uintptr(unsafe.Pointer(&out.Handles)))
	counters := struct {
		Size, PageFaults                                                                                                          uint32
		PeakWorkingSet, WorkingSet, QuotaPeakPaged, QuotaPaged, QuotaPeakNonPaged, QuotaNonPaged, Pagefile, PeakPagefile, Private uintptr
	}{}
	counters.Size = uint32(unsafe.Sizeof(counters))
	ok2, _, _ := memoryInfo.Call(uintptr(process), uintptr(unsafe.Pointer(&counters)), uintptr(counters.Size))
	out.WorkingSetBytes, out.PrivateBytes = uint64(counters.WorkingSet), uint64(counters.Private)
	out.ProcessMeasured = ok1 != 0 && ok2 != 0
	var created, exited, kernel, user windows.Filetime
	if windows.GetProcessTimes(process, &created, &exited, &kernel, &user) == nil {
		out.CPUMs = float64(uint64(kernel.HighDateTime)<<32|uint64(kernel.LowDateTime))/10000 +
			float64(uint64(user.HighDateTime)<<32|uint64(user.LowDateTime))/10000
		out.CPUMeasured = true
	}
	out.LogicalCPUs = runtime.NumCPU()
	return out
}
