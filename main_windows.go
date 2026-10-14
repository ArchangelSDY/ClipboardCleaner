//go:build windows

package main

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	shell32                 = syscall.NewLazyDLL("shell32.dll")
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	registerClass           = user32.NewProc("RegisterClassExW")
	createWindow            = user32.NewProc("CreateWindowExW")
	defWindowProc           = user32.NewProc("DefWindowProcW")
	destroyWindow           = user32.NewProc("DestroyWindow")
	getMessage              = user32.NewProc("GetMessageW")
	translateMessage        = user32.NewProc("TranslateMessage")
	dispatchMessage         = user32.NewProc("DispatchMessageW")
	postQuitMessage         = user32.NewProc("PostQuitMessage")
	postMessage             = user32.NewProc("PostMessageW")
	registerWindowMessage   = user32.NewProc("RegisterWindowMessageW")
	loadIcon                = user32.NewProc("LoadIconW")
	setTimer                = user32.NewProc("SetTimer")
	killTimer               = user32.NewProc("KillTimer")
	addClipboardListener    = user32.NewProc("AddClipboardFormatListener")
	removeClipboardListener = user32.NewProc("RemoveClipboardFormatListener")
	getClipboardSequence    = user32.NewProc("GetClipboardSequenceNumber")
	openClipboard           = user32.NewProc("OpenClipboard")
	emptyClipboard          = user32.NewProc("EmptyClipboard")
	closeClipboard          = user32.NewProc("CloseClipboard")
	createPopupMenu         = user32.NewProc("CreatePopupMenu")
	appendMenu              = user32.NewProc("AppendMenuW")
	destroyMenu             = user32.NewProc("DestroyMenu")
	getCursorPos            = user32.NewProc("GetCursorPos")
	setForegroundWindow     = user32.NewProc("SetForegroundWindow")
	trackPopupMenu          = user32.NewProc("TrackPopupMenu")
	messageBox              = user32.NewProc("MessageBoxW")
	notifyIcon              = shell32.NewProc("Shell_NotifyIconW")
	getModuleHandle         = kernel32.NewProc("GetModuleHandleW")
)

const (
	wmDestroy         = 0x0002
	wmClose           = 0x0010
	wmTimer           = 0x0113
	wmClipboardUpdate = 0x031D
	wmTray            = 0x8001
	wmLeftButtonUp    = 0x0202
	wmRightButtonUp   = 0x0205
	wmContextMenu     = 0x007B
	trayAdd           = 0
	trayModify        = 1
	trayDelete        = 2
	trayMessage       = 1
	trayIcon          = 2
	trayTip           = 4
	trayInfo          = 0x10
	exitCommand       = 1
)

type point struct{ x, y int32 }

type message struct {
	window         uintptr
	id             uint32
	wParam, lParam uintptr
	timestamp      uint32
	position       point
	private        uint32
}

type windowClass struct {
	size, style                        uint32
	procedure                          uintptr
	classExtra, windowExtra            int32
	instance, icon, cursor, background uintptr
	menuName, className                *uint16
	smallIcon                          uintptr
}

type notificationIcon struct {
	size                uint32
	window              uintptr
	id, flags, callback uint32
	icon                uintptr
	tip                 [128]uint16
	state, stateMask    uint32
	info                [256]uint16
	timeout             uint32
	title               [64]uint16
	infoFlags           uint32
	guid                [16]byte
	balloonIcon         uintptr
}

type application struct {
	window                 uintptr
	icon                   notificationIcon
	policy                 clipboardPolicy
	taskbarCreated         uint32
	listening, timer, tray bool
	failureSince           time.Time
	failureNotified        bool
}

var app application

func wide(s string) *uint16 {
	return syscall.StringToUTF16Ptr(s)
}

func setText(dst []uint16, text string) {
	for i := range dst {
		dst[i] = 0
	}
	src := syscall.StringToUTF16(text)
	copy(dst[:len(dst)-1], src[:len(src)-1])
}

func apiError(name string, err error) error {
	if err == syscall.Errno(0) {
		return fmt.Errorf("%s failed", name)
	}
	return fmt.Errorf("%s: %w", name, err)
}

func showFatal(err error) {
	messageBox.Call(0, uintptr(unsafe.Pointer(wide(err.Error()))),
		uintptr(unsafe.Pointer(wide("Clipboard Cleaner"))), 0x10)
}

func sequenceNumber() uint32 {
	value, _, _ := getClipboardSequence.Call()
	return uint32(value)
}

func (a *application) installTray() error {
	result, _, err := notifyIcon.Call(trayAdd, uintptr(unsafe.Pointer(&a.icon)))
	if result == 0 {
		return apiError("Adding tray icon", err)
	}
	a.tray = true
	return nil
}

func (a *application) reportFailure(err error, now time.Time) {
	if a.failureSince.IsZero() {
		a.failureSince = now
	}
	if a.failureNotified || now.Sub(a.failureSince) < clipboardTimeout {
		return
	}
	a.failureNotified = true
	a.icon.flags |= trayInfo
	setText(a.icon.tip[:], "Clipboard Cleaner: unable to clear; retrying")
	setText(a.icon.title[:], "Clipboard could not be cleared")
	setText(a.icon.info[:], err.Error()+". Retrying; another application may be holding the clipboard.")
	a.icon.infoFlags = 2
	result, _, _ := notifyIcon.Call(trayModify, uintptr(unsafe.Pointer(&a.icon)))
	a.icon.flags &^= trayInfo
	if result == 0 {
		showFatal(err)
	}
}

func (a *application) resetFailure() {
	if a.failureNotified {
		setText(a.icon.tip[:], "Clipboard Cleaner: clears after 5 seconds")
		notifyIcon.Call(trayModify, uintptr(unsafe.Pointer(&a.icon)))
	}
	a.failureSince = time.Time{}
	a.failureNotified = false
}

func (a *application) observe(now time.Time) {
	sequence := sequenceNumber()
	if sequence != a.policy.sequence {
		a.resetFailure()
	}
	a.policy.observe(sequence, now)
}

func (a *application) tick() {
	now := time.Now()
	a.observe(now)
	if !a.policy.due(now) {
		return
	}
	result, _, err := openClipboard.Call(a.window)
	if result == 0 {
		a.reportFailure(apiError("OpenClipboard", err), now)
		return
	}
	// Recheck the sequence under the clipboard lock to protect a fresh copy.
	sequence := sequenceNumber()
	if sequence != a.policy.sequence {
		a.resetFailure()
	}
	var clearErr error
	if a.policy.mayClear(sequence, time.Now()) {
		result, _, err = emptyClipboard.Call()
		if result == 0 {
			clearErr = apiError("EmptyClipboard", err)
		} else {
			a.policy.cleared(sequenceNumber())
			a.resetFailure()
		}
	}
	result, _, err = closeClipboard.Call()
	if result == 0 {
		// Continuing after a failed release could lock out other applications.
		showFatal(apiError("CloseClipboard", err))
		destroyWindow.Call(a.window)
		return
	}
	if clearErr != nil {
		a.reportFailure(clearErr, now)
	}
}

func (a *application) showMenu() {
	menu, _, err := createPopupMenu.Call()
	if menu == 0 {
		showFatal(apiError("CreatePopupMenu", err))
		return
	}
	defer destroyMenu.Call(menu)
	result, _, err := appendMenu.Call(menu, 0, exitCommand, uintptr(unsafe.Pointer(wide("Exit"))))
	if result == 0 {
		showFatal(apiError("AppendMenu", err))
		return
	}
	var position point
	result, _, err = getCursorPos.Call(uintptr(unsafe.Pointer(&position)))
	if result == 0 {
		showFatal(apiError("GetCursorPos", err))
		return
	}
	setForegroundWindow.Call(a.window)
	command, _, _ := trackPopupMenu.Call(menu, 0x0100|0x0080|0x0002,
		uintptr(position.x), uintptr(position.y), 0, a.window, 0)
	postMessage.Call(a.window, 0, 0, 0)
	if command == exitCommand {
		destroyWindow.Call(a.window)
	}
}

func windowProcedure(window uintptr, id uint32, wParam, lParam uintptr) uintptr {
	switch id {
	case wmClipboardUpdate:
		app.observe(time.Now())
		return 0
	case wmTimer:
		if wParam == 1 {
			app.tick()
		}
		return 0
	case wmTray:
		switch uint32(lParam) {
		case wmLeftButtonUp, wmRightButtonUp, wmContextMenu:
			app.showMenu()
		}
		return 0
	case wmClose:
		destroyWindow.Call(window)
		return 0
	case wmDestroy:
		if app.timer {
			killTimer.Call(window, 1)
		}
		if app.listening {
			removeClipboardListener.Call(window)
		}
		if app.tray {
			notifyIcon.Call(trayDelete, uintptr(unsafe.Pointer(&app.icon)))
		}
		app.window = 0
		postQuitMessage.Call(0)
		return 0
	}
	if app.taskbarCreated != 0 && id == app.taskbarCreated {
		app.tray = false
		if err := app.installTray(); err != nil {
			showFatal(err)
			destroyWindow.Call(window)
		}
		return 0
	}
	result, _, _ := defWindowProc.Call(window, uintptr(id), wParam, lParam)
	return result
}

func run() error {
	instance, _, err := getModuleHandle.Call(0)
	if instance == 0 {
		return apiError("GetModuleHandle", err)
	}
	className := wide("ClipboardCleanerWindow")
	class := windowClass{
		procedure: syscall.NewCallback(windowProcedure),
		instance:  instance,
		className: className,
	}
	class.size = uint32(unsafe.Sizeof(class))
	result, _, err := registerClass.Call(uintptr(unsafe.Pointer(&class)))
	if result == 0 {
		return apiError("RegisterClassEx", err)
	}
	app.window, _, err = createWindow.Call(0, uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(wide("Clipboard Cleaner"))), 0, 0, 0, 0, 0, 0, 0, instance, 0)
	if app.window == 0 {
		return apiError("CreateWindowEx", err)
	}
	defer func() {
		if app.window != 0 {
			destroyWindow.Call(app.window)
		}
	}()
	result, _, err = registerWindowMessage.Call(uintptr(unsafe.Pointer(wide("TaskbarCreated"))))
	if result == 0 {
		return apiError("RegisterWindowMessage", err)
	}
	app.taskbarCreated = uint32(result)
	icon, _, err := loadIcon.Call(0, 32512)
	if icon == 0 {
		return apiError("LoadIcon", err)
	}
	app.icon = notificationIcon{
		window:   app.window,
		id:       1,
		flags:    trayMessage | trayIcon | trayTip,
		callback: wmTray,
		icon:     icon,
	}
	app.icon.size = uint32(unsafe.Sizeof(app.icon))
	setText(app.icon.tip[:], "Clipboard Cleaner: clears after 5 seconds")
	if err := app.installTray(); err != nil {
		return err
	}
	result, _, err = addClipboardListener.Call(app.window)
	if result == 0 {
		return apiError("AddClipboardFormatListener", err)
	}
	app.listening = true
	app.policy = newClipboardPolicy(sequenceNumber(), time.Now())
	result, _, err = setTimer.Call(app.window, 1, 100, 0)
	if result == 0 {
		return apiError("SetTimer", err)
	}
	app.timer = true
	var msg message
	for {
		result, _, err = getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(result) == -1 {
			return apiError("GetMessage", err)
		}
		if result == 0 {
			return nil
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		showFatal(err)
		os.Exit(1)
	}
}
