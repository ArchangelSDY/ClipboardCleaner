# Clipboard Cleaner

A small Windows-only Go application that clears the current shared clipboard
after **10 seconds without a new copy** by default. It uses native Win32 clipboard
notifications, a hidden window, a notification-area (tray) icon, and a native
popup menu. It never reads or logs clipboard contents and has no dependencies
outside the Go standard library.

![Clipboard Cleaner icon](assets/clipboard-cleaner.png)

## Build and run

Requires Windows 10/11 and Go 1.19 or newer. In PowerShell:

```powershell
Set-Location D:\Projects\ClipboardCleaner
go test ./...
go build -trimpath -ldflags="-H=windowsgui" -o ClipboardCleaner.exe .
.\ClipboardCleaner.exe
```

The executable has no console window and starts clearing automatically.
Click or right-click its tray icon for **Start**, **Stop**, **Timeout...**, and **Exit**.
**Stop** pauses clipboard clearing but leaves the app and tray icon running.
**Start** resumes clearing, giving the current clipboard a full grace period
using the selected timeout. The unavailable action is disabled, and the tray tooltip shows
whether the cleaner is running or stopped. Windows may place the icon in the
tray overflow. **Exit** closes the app and removes the tray icon; stopping or
exiting does not clear the clipboard immediately.
No startup registration or background service is installed.

**Timeout...** opens a native settings window. Enter a whole number of seconds
from **1 to 9223372036** and select **Apply**. Zero, negative, fractional, and
overflowing values are rejected visibly. Applying restarts the countdown for
the current clipboard with the new duration; a stopped cleaner stays stopped.
Cancel or closing the window leaves the setting unchanged. Clearing continues
while the settings window is open unless you use **Stop** first. The setting is
kept only in memory: every launch returns to **10 seconds**.

Existing clipboard contents get a 10-second grace period at launch. Every
clipboard change resets the timer. A 100 ms native timer checks expiration;
clearing occurs at or shortly after the configured timeout, subject to Windows scheduling.
The sequence is rechecked with the clipboard locked so a fresh copy cannot be
deleted by an expired timer. If another application holds the clipboard, the
cleaner retries without blocking the UI. Failures lasting five additional
seconds produce a native tray warning. Startup and fatal errors use a native
dialog.

## Privacy limits

Clearing the active clipboard removes its current formats, not **Windows
clipboard history**, pinned items, cloud-synced copies, or copies saved by
third-party clipboard managers. Disable history/cloud sync and configure any
clipboard managers separately if those retained copies are a concern. This
program reduces stale-paste risk; it cannot guarantee removal of data already
read or retained by another application. Run only one instance.

Tests exercise expiration, reset, lock-time sequence changes, self-generated
clears, sequence wraparound, contention retries, stop/start grace periods,
timeout validation, and runtime timeout changes without touching the actual
clipboard.

## Icon artwork

The original icon combines a clipboard, eraser, and clean sparkle. Its transparent
ICO includes sizes from 16 to 256 pixels, used by the tray and executable.
Editable vector artwork is in `assets\clipboard-cleaner.svg`. To regenerate
the matching raster artwork and the embedded amd64 Windows resource:

```powershell
.\scripts\draw-icon.ps1
go generate
go build -trimpath -ldflags="-H=windowsgui" -o ClipboardCleaner.exe .
```

The resource compiler (`github.com/akavel/rsrc`, pinned to v0.10.2) is a
generation-only tool; normal builds use the included `icon_windows_amd64.syso`
and require no downloads or extra runtime dependencies. Other Windows target
architectures need a resource generated with the matching `-arch` and filename.
