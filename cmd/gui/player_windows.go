//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	libmpvDLLName    = "libmpv-2.dll"
	mainWindowTitle  = "Crunchyroll Downloader"
	playClassNameStr = "CrunchyrollDownloaderPlaySurface"

	mpvFormatFlag   = 3
	mpvFormatDouble = 5

	wsChild        = 0x40000000
	wsVisible      = 0x10000000
	wsClipSiblings = 0x04000000
	wsClipChildren = 0x02000000
	wsExNoActivate = 0x08000000

	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoActivate = 0x0010
	swpShowWindow = 0x0040

	hwndBottom     = 1
	blackBrush     = 4
	wmEraseBkgnd   = 0x0014
	wmNCHitTest    = 0x0084
	wmKeyDown      = 0x0100
	vkEscape       = 0x1B
	htTransparent  = ^uintptr(0) // (HWND)(LONG_PTR)-1
	wmPlayJob      = 0x8001      // WM_APP+1
	pmNoRemove     = 0
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")

	procCreateWindowExW    = user32.NewProc("CreateWindowExW")
	procRegisterClassExW   = user32.NewProc("RegisterClassExW")
	procDefWindowProcW     = user32.NewProc("DefWindowProcW")
	procDestroyWindow      = user32.NewProc("DestroyWindow")
	procMoveWindow         = user32.NewProc("MoveWindow")
	procSetWindowPos       = user32.NewProc("SetWindowPos")
	procFindWindowW        = user32.NewProc("FindWindowW")
	procGetDpiForWindow    = user32.NewProc("GetDpiForWindow")
	procIsWindow           = user32.NewProc("IsWindow")
	procPeekMessageW       = user32.NewProc("PeekMessageW")
	procGetMessageW        = user32.NewProc("GetMessageW")
	procTranslateMessage   = user32.NewProc("TranslateMessage")
	procDispatchMessageW   = user32.NewProc("DispatchMessageW")
	procPostThreadMessageW = user32.NewProc("PostThreadMessageW")
	procGetModuleHandleW   = kernel32.NewProc("GetModuleHandleW")
	procGetCurrentThreadId = kernel32.NewProc("GetCurrentThreadId")
	procGetStockObject     = gdi32.NewProc("GetStockObject")

	playClassUTF16 = windows.StringToUTF16Ptr(playClassNameStr)
	playWndProcCb  = syscall.NewCallback(playSurfaceWndProc)
	playClassOnce  sync.Once
	playClassErr   error

	hwndPumpOnce  sync.Once
	hwndPumpReady = make(chan struct{})
	hwndTID       uint32
	hwndJobMu     sync.Mutex
	hwndJobs      []hwndJob
)

type winPOINT struct {
	X, Y int32
}

type winMSG struct {
	Hwnd    windows.HWND
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      winPOINT
}

type hwndJob struct {
	fn   func() error
	done chan error
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

type libmpvProcs struct {
	dll              *windows.DLL
	create           *windows.Proc
	setOptionString  *windows.Proc
	initialize       *windows.Proc
	command          *windows.Proc
	commandString    *windows.Proc
	setProperty      *windows.Proc
	getProperty      *windows.Proc
	terminateDestroy *windows.Proc
}

type windowsMpvHost struct {
	mu     sync.Mutex
	procs  *libmpvProcs
	handle uintptr
	wid    uintptr
}

func openLibmpv(explicit string) (*windows.DLL, error) {
	if explicit != "" {
		dll, err := windows.LoadDLL(explicit)
		if err != nil {
			return nil, missingPlayerErr()
		}
		return dll, nil
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), libmpvDLLName)
		if dll, err := windows.LoadDLL(p); err == nil {
			return dll, nil
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		p := filepath.Join(cwd, libmpvDLLName)
		if dll, err := windows.LoadDLL(p); err == nil {
			return dll, nil
		}
	}
	dll, err := windows.LoadDLL(libmpvDLLName)
	if err != nil {
		return nil, missingPlayerErr()
	}
	return dll, nil
}

func bindLibmpv(dll *windows.DLL) (*libmpvProcs, error) {
	must := []string{
		"mpv_create",
		"mpv_set_option_string",
		"mpv_initialize",
		"mpv_set_property",
		"mpv_get_property",
		"mpv_terminate_destroy",
	}
	found := make(map[string]*windows.Proc, len(must)+4)
	for _, name := range must {
		p, err := dll.FindProc(name)
		if err != nil {
			_ = dll.Release()
			return nil, missingPlayerErr()
		}
		found[name] = p
	}
	lp := &libmpvProcs{
		dll:              dll,
		create:           found["mpv_create"],
		setOptionString:  found["mpv_set_option_string"],
		initialize:       found["mpv_initialize"],
		setProperty:      found["mpv_set_property"],
		getProperty:      found["mpv_get_property"],
		terminateDestroy: found["mpv_terminate_destroy"],
	}
	if p, err := dll.FindProc("mpv_command"); err == nil {
		lp.command = p
	}
	if p, err := dll.FindProc("mpv_command_string"); err == nil {
		lp.commandString = p
	}
	if lp.command == nil && lp.commandString == nil {
		_ = dll.Release()
		return nil, missingPlayerErr()
	}
	return lp, nil
}

func newMpvHost() (MpvHost, error) {
	dll, err := openLibmpv("")
	if err != nil {
		return nil, err
	}
	procs, err := bindLibmpv(dll)
	if err != nil {
		return nil, err
	}
	return &windowsMpvHost{procs: procs}, nil
}

func (h *windowsMpvHost) Attach(hwnd uintptr) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if hwnd == 0 {
		return missingPlayerErr()
	}
	if h.procs == nil {
		return missingPlayerErr()
	}
	if h.handle != 0 {
		if h.wid == hwnd {
			return nil
		}
		h.terminateLocked()
	}
	r, _, _ := h.procs.create.Call()
	if r == 0 {
		return missingPlayerErr()
	}
	h.handle = r
	h.wid = hwnd
	wid := strconv.FormatUint(uint64(hwnd), 10)
	opts := [][2]string{
		{"wid", wid},
		{"osc", "no"},
		{"input-default-bindings", "no"},
		{"input-vo-keyboard", "no"},
		{"idle", "yes"},
		{"keep-open", "yes"},
		{"force-window", "no"},
		{"config", "no"},
		{"terminal", "no"},
		{"vo", "gpu"},
	}
	for _, kv := range opts {
		if err := h.setOptionStringLocked(kv[0], kv[1]); err != nil {
			h.terminateLocked()
			return missingPlayerErr()
		}
	}
	st, _, _ := h.procs.initialize.Call(h.handle)
	if mpvStatus(st) < 0 {
		h.terminateLocked()
		return missingPlayerErr()
	}
	return nil
}

func (h *windowsMpvHost) LoadFile(path string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.handle == 0 {
		return missingPlayerErr()
	}
	return h.commandLocked("loadfile", path, "replace")
}

func (h *windowsMpvHost) AddAudio(path string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.handle == 0 {
		return missingPlayerErr()
	}
	return h.commandLocked("audio-add", path)
}

func (h *windowsMpvHost) Pause(p bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	var flag int32
	if p {
		flag = 1
	}
	return h.setPropertyLocked("pause", mpvFormatFlag, unsafe.Pointer(&flag))
}

func (h *windowsMpvHost) Seek(seconds float64) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	v := seconds
	return h.setPropertyLocked("time-pos", mpvFormatDouble, unsafe.Pointer(&v))
}

func (h *windowsMpvHost) Position() (float64, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.getDoubleLocked("time-pos")
}

func (h *windowsMpvHost) Duration() (float64, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.getDoubleLocked("duration")
}

func (h *windowsMpvHost) SetVolume(percent int) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	v := float64(percent)
	err := h.setPropertyLocked("volume", mpvFormatDouble, unsafe.Pointer(&v))
	runtime.KeepAlive(v)
	return err
}

func (h *windowsMpvHost) SetSpeed(rate float64) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if rate <= 0 {
		rate = 1
	}
	err := h.setPropertyLocked("speed", mpvFormatDouble, unsafe.Pointer(&rate))
	runtime.KeepAlive(rate)
	return err
}

func (h *windowsMpvHost) Destroy() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.terminateLocked()
	if h.procs != nil && h.procs.dll != nil {
		_ = h.procs.dll.Release()
		h.procs.dll = nil
	}
	h.procs = nil
	return nil
}

func (h *windowsMpvHost) playFlags() (paused bool, eof bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	paused, _ = h.getFlagLocked("pause")
	eof, _ = h.getFlagLocked("eof-reached")
	return paused, eof
}

func (h *windowsMpvHost) terminateLocked() {
	if h.handle != 0 && h.procs != nil && h.procs.terminateDestroy != nil {
		h.procs.terminateDestroy.Call(h.handle)
	}
	h.handle = 0
}

func (h *windowsMpvHost) setOptionStringLocked(name, value string) error {
	if h.handle == 0 || h.procs == nil {
		return missingPlayerErr()
	}
	n, err := syscall.BytePtrFromString(name)
	if err != nil {
		return err
	}
	v, err := syscall.BytePtrFromString(value)
	if err != nil {
		return err
	}
	st, _, _ := h.procs.setOptionString.Call(h.handle, uintptr(unsafe.Pointer(n)), uintptr(unsafe.Pointer(v)))
	runtime.KeepAlive(n)
	runtime.KeepAlive(v)
	if mpvStatus(st) < 0 {
		return h.mpvErrLocked(mpvStatus(st))
	}
	return nil
}

func (h *windowsMpvHost) commandLocked(args ...string) error {
	if h.handle == 0 || h.procs == nil {
		return missingPlayerErr()
	}
	if h.procs.command != nil {
		ptrs := make([]*byte, len(args)+1)
		for i, a := range args {
			p, err := syscall.BytePtrFromString(a)
			if err != nil {
				return err
			}
			ptrs[i] = p
		}
		argv := make([]uintptr, len(args)+1)
		for i, p := range ptrs {
			if p != nil {
				argv[i] = uintptr(unsafe.Pointer(p))
			}
		}
		st, _, _ := h.procs.command.Call(h.handle, uintptr(unsafe.Pointer(&argv[0])))
		runtime.KeepAlive(ptrs)
		runtime.KeepAlive(argv)
		if mpvStatus(st) < 0 {
			return h.mpvErrLocked(mpvStatus(st))
		}
		return nil
	}
	// Fallback: mpv_command_string with quoted path.
	quoted := make([]string, len(args))
	for i, a := range args {
		if i == 0 {
			quoted[i] = a
			continue
		}
		quoted[i] = `"` + a + `"`
	}
	cmd := quoted[0]
	for i := 1; i < len(quoted); i++ {
		cmd += " " + quoted[i]
	}
	p, err := syscall.BytePtrFromString(cmd)
	if err != nil {
		return err
	}
	st, _, _ := h.procs.commandString.Call(h.handle, uintptr(unsafe.Pointer(p)))
	runtime.KeepAlive(p)
	if mpvStatus(st) < 0 {
		return h.mpvErrLocked(mpvStatus(st))
	}
	return nil
}

func (h *windowsMpvHost) setPropertyLocked(name string, format int, data unsafe.Pointer) error {
	if h.handle == 0 || h.procs == nil {
		return missingPlayerErr()
	}
	n, err := syscall.BytePtrFromString(name)
	if err != nil {
		return err
	}
	st, _, _ := h.procs.setProperty.Call(h.handle, uintptr(unsafe.Pointer(n)), uintptr(format), uintptr(data))
	runtime.KeepAlive(n)
	if mpvStatus(st) < 0 {
		return h.mpvErrLocked(mpvStatus(st))
	}
	return nil
}

func (h *windowsMpvHost) getDoubleLocked(name string) (float64, error) {
	if h.handle == 0 || h.procs == nil {
		return 0, nil
	}
	n, err := syscall.BytePtrFromString(name)
	if err != nil {
		return 0, err
	}
	var v float64
	st, _, _ := h.procs.getProperty.Call(h.handle, uintptr(unsafe.Pointer(n)), mpvFormatDouble, uintptr(unsafe.Pointer(&v)))
	runtime.KeepAlive(n)
	if mpvStatus(st) < 0 {
		return 0, nil
	}
	return v, nil
}

func (h *windowsMpvHost) getFlagLocked(name string) (bool, error) {
	if h.handle == 0 || h.procs == nil {
		return false, nil
	}
	n, err := syscall.BytePtrFromString(name)
	if err != nil {
		return false, err
	}
	var flag int32
	st, _, _ := h.procs.getProperty.Call(h.handle, uintptr(unsafe.Pointer(n)), mpvFormatFlag, uintptr(unsafe.Pointer(&flag)))
	runtime.KeepAlive(n)
	if mpvStatus(st) < 0 {
		return false, nil
	}
	return flag != 0, nil
}

func (h *windowsMpvHost) mpvErrLocked(status int) error {
	return fmtMpvStatus(status, "")
}

func fmtMpvStatus(status int, msg string) error {
	if msg == "" {
		msg = "mpv error " + strconv.Itoa(status)
	}
	return &mpvStatusError{status: status, msg: msg}
}

type mpvStatusError struct {
	status int
	msg    string
}

func (e *mpvStatusError) Error() string {
	return e.msg
}

func mpvStatus(r uintptr) int {
	return int(int32(r))
}

func playSurfaceWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch uint32(msg) {
	case wmNCHitTest:
		// Clicks and hover must reach the WebView chrome (back, timeline), not mpv.
		return htTransparent
	case wmKeyDown:
		if wParam == vkEscape {
			notifyPlayEscape()
			return 0
		}
	case wmEraseBkgnd:
		return 1
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

func notifyPlayEscape() {
	fn := playEscapeFn
	if fn == nil {
		return
	}
	go fn()
}

func registerPlayClass() error {
	playClassOnce.Do(func() {
		inst, _, _ := procGetModuleHandleW.Call(0)
		brush, _, _ := procGetStockObject.Call(blackBrush)
		wcx := wndClassEx{
			WndProc:    playWndProcCb,
			Instance:   windows.Handle(inst),
			Background: windows.Handle(brush),
			ClassName:  playClassUTF16,
		}
		wcx.Size = uint32(unsafe.Sizeof(wcx))
		atom, _, lastErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcx)))
		if atom == 0 {
			playClassErr = lastErr
		}
	})
	return playClassErr
}

func findMainWindowHWND() windows.HWND {
	title, err := windows.UTF16PtrFromString(mainWindowTitle)
	if err == nil {
		r, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(title)))
		if r != 0 {
			return windows.HWND(r)
		}
	}
	return windows.GetForegroundWindow()
}

func isWindow(hwnd windows.HWND) bool {
	if hwnd == 0 {
		return false
	}
	r, _, _ := procIsWindow.Call(uintptr(hwnd))
	return r != 0
}

func dpiScale(hwnd windows.HWND) float64 {
	if hwnd == 0 || procGetDpiForWindow.Find() != nil {
		return 1
	}
	r, _, _ := procGetDpiForWindow.Call(uintptr(hwnd))
	if r == 0 {
		return 1
	}
	return float64(r) / 96.0
}

func clientPixels(parent windows.HWND, r PlayStageRect) (x, y, w, h int32) {
	scale := dpiScale(parent)
	x = int32(r.X * scale)
	y = int32(r.Y * scale)
	w = int32(r.W * scale)
	h = int32(r.H * scale)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return x, y, w, h
}

func u32(v int32) uintptr {
	return uintptr(uint32(v))
}

func raisePlaySurface(hwnd windows.HWND) {
	if hwnd == 0 {
		return
	}
	// Keep the video pane *under* WebView2 so HTML chrome receives mouse/keyboard.
	procSetWindowPos.Call(
		uintptr(hwnd),
		hwndBottom,
		0, 0, 0, 0,
		swpNoMove|swpNoSize|swpShowWindow|swpNoActivate,
	)
}

func startHWNDPump() {
	hwndPumpOnce.Do(func() {
		go hwndPumpLoop()
		<-hwndPumpReady
	})
}

func hwndPumpLoop() {
	runtime.LockOSThread()
	var msg winMSG
	procPeekMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, pmNoRemove)
	r, _, _ := procGetCurrentThreadId.Call()
	hwndTID = uint32(r)
	close(hwndPumpReady)

	for {
		got, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(got) <= 0 {
			return
		}
		if msg.Message == wmPlayJob {
			runHWNDJobs()
			continue
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func runHWNDJobs() {
	hwndJobMu.Lock()
	jobs := hwndJobs
	hwndJobs = nil
	hwndJobMu.Unlock()
	for _, job := range jobs {
		err := func() (err error) {
			defer func() {
				if rec := recover(); rec != nil {
					err = fmt.Errorf("play hwnd job panic: %v", rec)
				}
			}()
			return job.fn()
		}()
		job.done <- err
	}
}

func runOnHWNDThread(fn func() error) error {
	startHWNDPump()
	job := hwndJob{fn: fn, done: make(chan error, 1)}
	hwndJobMu.Lock()
	hwndJobs = append(hwndJobs, job)
	hwndJobMu.Unlock()
	posted, _, err := procPostThreadMessageW.Call(uintptr(hwndTID), wmPlayJob, 0, 0)
	if posted == 0 {
		hwndJobMu.Lock()
		kept := hwndJobs[:0]
		for _, j := range hwndJobs {
			if j.done != job.done {
				kept = append(kept, j)
			}
		}
		hwndJobs = kept
		hwndJobMu.Unlock()
		return fmt.Errorf("post play hwnd job: %v", err)
	}
	return <-job.done
}

func (a *App) ensurePlaySurfaceLocked() (uintptr, error) {
	parent := windows.HWND(a.playParent)
	if !isWindow(parent) {
		parent = findMainWindowHWND()
		a.playParent = uintptr(parent)
	}
	if parent == 0 {
		return 0, missingPlayerErr()
	}
	rect := a.playRect
	err := runOnHWNDThread(func() error {
		return a.ensurePlaySurfaceOnPump(parent, rect)
	})
	if err != nil {
		return 0, err
	}
	if a.playChild == 0 {
		return 0, missingPlayerErr()
	}
	return a.playChild, nil
}

func (a *App) ensurePlaySurfaceOnPump(parent windows.HWND, rect PlayStageRect) error {
	if a.playChild != 0 && isWindow(windows.HWND(a.playChild)) {
		return a.movePlaySurfaceOnPump(parent, windows.HWND(a.playChild), rect)
	}
	if err := registerPlayClass(); err != nil {
		return err
	}
	x, y, w, h := clientPixels(parent, rect)
	inst, _, _ := procGetModuleHandleW.Call(0)
	style := uintptr(wsChild | wsVisible | wsClipSiblings | wsClipChildren)
	r, _, lastErr := procCreateWindowExW.Call(
		wsExNoActivate,
		uintptr(unsafe.Pointer(playClassUTF16)),
		0,
		style,
		u32(x), u32(y), u32(w), u32(h),
		uintptr(parent),
		0,
		inst,
		0,
	)
	if r == 0 {
		return lastErr
	}
	a.playChild = r
	raisePlaySurface(windows.HWND(r))
	return nil
}

func (a *App) movePlaySurfaceLocked() error {
	if a.playChild == 0 {
		return nil
	}
	parent := windows.HWND(a.playParent)
	child := windows.HWND(a.playChild)
	rect := a.playRect
	return runOnHWNDThread(func() error {
		return a.movePlaySurfaceOnPump(parent, child, rect)
	})
}

func (a *App) movePlaySurfaceOnPump(parent, hwnd windows.HWND, rect PlayStageRect) error {
	if hwnd == 0 || !isWindow(hwnd) {
		return nil
	}
	x, y, w, h := clientPixels(parent, rect)
	procMoveWindow.Call(uintptr(hwnd), u32(x), u32(y), u32(w), u32(h), 1)
	raisePlaySurface(hwnd)
	return nil
}

func (a *App) destroyPlaySurfaceLocked() error {
	if a.playChild == 0 {
		return nil
	}
	return runOnHWNDThread(func() error {
		hwnd := a.playChild
		r, _, err := procDestroyWindow.Call(hwnd)
		if r != 0 {
			a.playChild = 0
			return nil
		}
		if !isWindow(windows.HWND(hwnd)) {
			a.playChild = 0
			return nil
		}
		return err
	})
}
