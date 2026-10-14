# Clipboard Cleaner

A small Windows-only Go application that clears the current shared clipboard
after **five seconds without a new copy**. It uses native Win32 clipboard
notifications, a hidden window, a notification-area (tray) icon, and an **Exit**
popup menu. It never reads or logs clipboard contents and has no dependencies
outside the Go standard library.

## Build and run

Requires Windows 10/11 and Go 1.19 or newer. In PowerShell:

```powershell
Set-Location D:\Projects\ClipboardCleaner
go test ./...
go build -trimpath -ldflags="-H=windowsgui" -o ClipboardCleaner.exe .
.\ClipboardCleaner.exe
```

The executable has no console window. Click or right-click its tray icon and
select **Exit** to stop it. Windows may place the icon in the tray overflow.
Exiting removes the tray icon; it does not clear the clipboard immediately.
No startup registration or background service is installed.

Existing clipboard contents get a five-second grace period at launch. Every
clipboard change resets the timer. A 100 ms native timer checks expiration;
clearing occurs at or shortly after five seconds, subject to Windows scheduling.
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
clears, sequence wraparound, and contention retries without touching the actual
clipboard.
