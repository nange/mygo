# Tray menu

Run from the repository root:

```sh
go run ./examples/tray-menu
```

The window lists servers. The same servers are the items of the **Servers**
submenu of the tray icon, in the menu bar (macOS) or notification area
(Windows and Linux). Edit a name in the window to rename its menu item, and
use **Add server**, **Remove** or **Remove last** to change the submenu.

The tray menu does the same: **Add Server** appends an item, **Rename Active
Server** gives the active server another name, and **Remove Last Server**
removes one. **Show Window** brings the window back after closing it, which
hides it rather than quitting the app (if there is no tray icon, closing the
window quits as usual). The menu's disabled last line shows the last change,
and the submenu's title counts the servers.

Changes to labels and checks show in the menu by themselves. After adding or
removing items, the example calls `Tray.SetMenu` with the menu again: a tray
shows the structure it was given last. That is the same call that would
replace the whole menu.

On Windows the menu opens on a right-click of the icon. On Linux the icon
needs `libayatana-appindicator3`; without it `NewTray` reports an error, the
window still opens, and the same list is there to use. On macOS the icon is a
template image, so it follows the menu bar's appearance.

The example needs Go alone, with no frontend build or webview. To develop
with live reload or package it:

```sh
go run ./cmd/mygo dev examples/tray-menu
go run ./cmd/mygo build examples/tray-menu
```
