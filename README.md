# File Drop

File Drop receives files from Android's native Quick Share. Every transfer
requires confirmation in the PaNasMs panel after comparing the PIN. A Galaxy Z
Fold7 running Android 16 completed a transfer with the locally installed 0.1.5
build; the published build has not yet been tested on that phone.

## Accepted requirements

- Receive through native Quick Share without a companion Android application.
- The default inbox is the receiving user's actual Linux home plus `Quick Share`.
  A home relocation must update this default automatically.
- Users can change only their own inbox; administrator visibility of devices does
  not grant permission to change another user's inbox through this module.
- Write incoming files as their owning Linux user, including supplementary groups
  and the system umask. Never write user payloads as root.
- Each request requires confirmation in the PaNasMs panel after comparing the
  PIN with the sender's device. Remembered devices are deferred until persistent
  identity can be authenticated; display names and network addresses are not trust.
- Future device grants must not pass to replacement accounts; require a durable
  core principal and UID before implementing remembered devices. This version
  stores no device grants. It identifies a receiving window and inbox by the
  current Linux username and UID, plus the principal where one already exists.
- Transfers run in the background. Preserve both files on a filename conflict.
- Discovery must not scan inboxes or wake data disks.
- AirDrop is outside the first implementation.

## First version workflow

Open File Drop and start a ten-minute receiving window. Select PaNasMs in Android
Quick Share. A request appears in the panel for up to one minute with filenames,
sizes and a PIN. Compare the PIN on the phone and in the panel, then accept or
decline. The window closes after one accepted transfer. Only one user's receiving
window can be active at a time, so the request belongs to that user.

Files are streamed to a child process running as the destination Linux account.
The worker rejects symbolic links in the destination path, refuses unsafe names,
checks announced sizes and adds a suffix when a filename already exists. The
default inbox follows the account's current home. A personal folder picker offers
other folders within that home.

The transport is a locally patched copy of ACLzz/qshare. It advertises
over BLE and mDNS on a shared LAN. Offline Wi-Fi Direct and fallback AP have not
been verified. No device identity or sender name is authenticated by this engine.
It does not store or display remembered devices.

The core and SDK declare `memberAccess` for specific modules. File Drop checks
the current account on every request and uses the shared personal folder picker.
The interface follows the canonical
[interface standard](https://github.com/PaNasMs/panasms/blob/main/docs/ui-design-guidelines.md).
Support English, Russian and Ukrainian and all four themes.

## Checks

```sh
go test ./...
```

The tests cover role isolation, explicit approval, unsafe filenames, size
checks, conflict preservation and symlink rejection. The physical transfer
was reported successful by the owner; received bytes, file ownership and
permissions were not independently checked after that transfer. Interrupted
transfers, other Android models and persistent device trust remain unverified.
