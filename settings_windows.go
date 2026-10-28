//go:build windows

package main

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

var (
	sendMessage         = user32.NewProc("SendMessageW")
	setWindowText       = user32.NewProc("SetWindowTextW")
	getWindowText       = user32.NewProc("GetWindowTextW")
	getWindowTextLength = user32.NewProc("GetWindowTextLengthW")
	setFocus            = user32.NewProc("SetFocus")
	isDialogMessage     = user32.NewProc("IsDialogMessageW")
	adjustWindowRect    = user32.NewProc("AdjustWindowRectEx")
	getStockObject      = syscall.NewLazyDLL("gdi32.dll").NewProc("GetStockObject")
)

func settingsProcedure(window uintptr, id uint32, wParam, lParam uintptr) uintptr {
	switch id {
	case 0x0111: // WM_COMMAND
		switch wParam & 0xffff {
		case 1:
			length, _, _ := getWindowTextLength.Call(app.timeoutEdit)
			text := make([]uint16, int(length)+1)
			getWindowText.Call(app.timeoutEdit, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)))
			timeout, err := parseTimeout(syscall.UTF16ToString(text))
			if err != nil {
				setWindowText.Call(app.settingsError, uintptr(unsafe.Pointer(wide(err.Error()))))
				setFocus.Call(app.timeoutEdit)
				sendMessage.Call(app.timeoutEdit, 0x00B1, 0, ^uintptr(0))
				return 0
			}
			app.policy.setTimeout(timeout, sequenceNumber(), time.Now())
			app.resetFailure()
			app.updateStatus()
			destroyWindow.Call(window)
			return 0
		case 2:
			destroyWindow.Call(window)
			return 0
		}
	case wmClose:
		destroyWindow.Call(window)
		return 0
	case wmDestroy:
		app.settings = 0
		app.timeoutEdit = 0
		app.settingsError = 0
		return 0
	}
	result, _, _ := defWindowProc.Call(window, uintptr(id), wParam, lParam)
	return result
}

func (a *application) showSettings() error {
	if a.settings != 0 {
		setForegroundWindow.Call(a.settings)
		return nil
	}
	instance, _, err := getModuleHandle.Call(0)
	if instance == 0 {
		return apiError("GetModuleHandle", err)
	}
	className := wide("ClipboardCleanerSettings")
	if !a.settingsClassRegistered {
		class := windowClass{
			procedure:  syscall.NewCallback(settingsProcedure),
			instance:   instance,
			className:  className,
			background: 16, // COLOR_BTNFACE + 1
			icon:       a.icon.icon,
		}
		class.size = uint32(unsafe.Sizeof(class))
		result, _, err := registerClass.Call(uintptr(unsafe.Pointer(&class)))
		if result == 0 {
			return apiError("Register settings window", err)
		}
		a.settingsClassRegistered = true
	}
	const style = 0x10C80000 // Visible caption and system menu, fixed size.
	rect := struct{ left, top, right, bottom int32 }{right: 430, bottom: 190}
	result, _, err := adjustWindowRect.Call(uintptr(unsafe.Pointer(&rect)), style, 0, 1)
	if result == 0 {
		return apiError("Size settings window", err)
	}
	window, _, err := createWindow.Call(1, uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(wide("Clipboard Cleaner - Timeout"))), style,
		0x80000000, 0x80000000, uintptr(rect.right-rect.left), uintptr(rect.bottom-rect.top),
		a.window, 0, instance, 0)
	if window == 0 {
		return apiError("Create settings window", err)
	}
	a.settings = window
	font, _, _ := getStockObject.Call(17) // DEFAULT_GUI_FONT
	control := func(class, text string, style uintptr, x, y, width, height int, id uintptr) (uintptr, error) {
		handle, _, err := createWindow.Call(0, uintptr(unsafe.Pointer(wide(class))),
			uintptr(unsafe.Pointer(wide(text))), style|0x50000000,
			uintptr(x), uintptr(y), uintptr(width), uintptr(height), window, id, instance, 0)
		if handle == 0 {
			return 0, apiError("Create settings control", err)
		}
		sendMessage.Call(handle, 0x0030, font, 1)
		return handle, nil
	}
	controls := []struct {
		class, text         string
		style               uintptr
		x, y, width, height int
		id                  uintptr
		dest                *uintptr
	}{
		{"STATIC", "Timeout in &seconds:", 0, 16, 18, 145, 22, 0, nil},
		{"EDIT", fmt.Sprintf("%d", a.policy.timeout/time.Second), 0x00810080, 165, 15, 245, 26, 100, &a.timeoutEdit},
		{"STATIC", "Changes apply for this run only. Applying restarts the countdown.\r\nClipboard clearing continues while this window is open.", 0, 16, 54, 400, 38, 0, nil},
		{"STATIC", "", 0, 16, 96, 400, 32, 101, &a.settingsError},
		{"BUTTON", "&Apply", 0x00010001, 225, 145, 88, 28, 1, nil},
		{"BUTTON", "Cancel", 0x00010000, 322, 145, 88, 28, 2, nil},
	}
	for _, spec := range controls {
		handle, err := control(spec.class, spec.text, spec.style, spec.x, spec.y, spec.width, spec.height, spec.id)
		if err != nil {
			destroyWindow.Call(window)
			return err
		}
		if spec.dest != nil {
			*spec.dest = handle
		}
	}
	sendMessage.Call(a.timeoutEdit, 0x00C5, 64, 0) // EM_SETLIMITTEXT
	setForegroundWindow.Call(window)
	setFocus.Call(a.timeoutEdit)
	sendMessage.Call(a.timeoutEdit, 0x00B1, 0, ^uintptr(0))
	return nil
}
