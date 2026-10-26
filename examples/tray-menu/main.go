// Tray-menu is a window and a tray icon whose menu changes while the app
// runs: the Servers submenu gains items, loses them and shows new labels as
// the window or the menu itself changes them. It needs Go alone, with no
// frontend build or webview.
//
//	go run ./examples/tray-menu
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"runtime"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// cities name the servers added one after another.
var cities = []string{"Tokyo", "Osaka", "Kyoto", "Sapporo", "Fukuoka", "Nagoya", "Sendai"}

// server is an item of the Servers submenu. All state is read and changed
// on the main thread, by the view, the menu items and the tray.
type server struct {
	id   int
	name string
	item *mygo.MenuItem
}

// title is the name the menu shows: an empty field shows the server's
// number instead of a blank item.
func (s *server) title() string {
	if name := strings.TrimSpace(s.name); name != "" {
		return name
	}
	return fmt.Sprintf("Server %d", s.id)
}

type app struct {
	win  *mygo.Window
	tray *mygo.Tray

	menu        *mygo.Menu
	serversItem *mygo.MenuItem
	serversMenu *mygo.Menu
	renameItem  *mygo.MenuItem
	removeItem  *mygo.MenuItem
	statusItem  *mygo.MenuItem

	servers  []*server
	nextID   int
	nextCity int
	status   string
	quitting bool
}

// view builds the window from the state, on the main thread.
func (a *app) view(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(24).Gap(12).Children(func() {
		ui.Text(c, "Tray menu").FontSize(26).Bold()
		ui.Text(c, "The icon in the menu bar (macOS) or notification area (Windows and Linux) opens a menu whose Servers submenu is built from this list. Edit a name to rename its item, and add or remove servers here or in the menu.").
			Wrap().TextColor(t.TextMuted)
		ui.Divider(c)

		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Textf(c, "Servers (%d)", len(a.servers)).FontSize(16).Bold().Grow(1)
			if ui.PrimaryButton(c.Key("add-server"), "Add server").Label("Add a server").Clicked() {
				a.addServer(a.nextCityName())
			}
		})
		for _, s := range slices.Clone(a.servers) {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if ui.TextInput(c.Key(fmt.Sprintf("name-%d", s.id)), &s.name).
					Label("Name of " + s.title()).Placeholder("Server name").Grow(1).Changed() {
					s.item.SetLabel(s.title())
					if s.item.IsChecked() {
						a.setStatus("Active server: %s", s.title())
					} else if a.win != nil {
						a.win.Invalidate()
					}
				}
				if ui.Button(c.Key(fmt.Sprintf("use-%d", s.id)), "Use").Label("Use " + s.title()).
					Disabled(s.item.IsChecked()).Clicked() {
					s.item.SetChecked(true)
					a.setStatus("Active server: %s", s.title())
				}
				if ui.Button(c.Key(fmt.Sprintf("remove-%d", s.id)), "Remove").Label("Remove " + s.title()).Clicked() {
					a.removeServer(s)
				}
			})
		}
		if len(a.servers) == 0 {
			ui.Text(c, "No servers. Add one from here or from the tray menu.").TextColor(t.TextMuted)
		}

		ui.Divider(c)
		active := "No active server"
		if s := a.selected(); s != nil {
			active = "Active: " + s.title()
		}
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, active).Grow(1).FontSize(12).TextColor(t.TextMuted)
			if ui.Button(c.Key("remove-last"), "Remove last").Disabled(len(a.servers) == 0).Clicked() {
				a.removeLast()
			}
		})
		ui.Text(c, a.status).Wrap().FontSize(12).TextColor(t.TextMuted)
	})
}

// buildMenu makes the tray menu and keeps the items the app changes.
func (a *app) buildMenu() {
	a.menu = mygo.NewMenu([]*mygo.MenuItem{
		{ID: "show", Label: "Show Window", Click: func(*mygo.MenuItem, *mygo.Window) { a.show() }},
		mygo.Separator(),
		{ID: "servers", Label: "Servers", Submenu: []*mygo.MenuItem{}},
		{ID: "add", Label: "Add Server", Click: func(*mygo.MenuItem, *mygo.Window) {
			a.addServer(a.nextCityName())
		}},
		{ID: "rename", Label: "Rename Active Server", Click: func(*mygo.MenuItem, *mygo.Window) {
			a.renameActive()
		}},
		{ID: "remove", Label: "Remove Last Server", Click: func(*mygo.MenuItem, *mygo.Window) {
			a.removeLast()
		}},
		mygo.Separator(),
		{ID: "status", Label: "Ready", Disabled: true},
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	})
	a.serversItem = a.menu.ItemByID("servers")
	a.serversMenu = a.serversItem.Menu()
	a.renameItem = a.menu.ItemByID("rename")
	a.removeItem = a.menu.ItemByID("remove")
	a.statusItem = a.menu.ItemByID("status")
}

// nextCityName returns the next name for a new server.
func (a *app) nextCityName() string {
	name := cities[a.nextCity%len(cities)]
	a.nextCity++
	return name
}

// addServer adds an item to the Servers submenu.
func (a *app) addServer(name string) *server {
	a.nextID++
	s := &server{id: a.nextID, name: name}
	s.item = &mygo.MenuItem{
		ID:      fmt.Sprintf("server-%d", s.id),
		Label:   s.title(),
		Type:    mygo.MenuItemRadio,
		Checked: len(a.servers) == 0,
		Click: func(*mygo.MenuItem, *mygo.Window) {
			a.setStatus("Active server: %s", s.title())
		},
	}
	a.servers = append(a.servers, s)
	a.serversMenu.Append(s.item)
	a.syncMenu()
	a.setStatus("Added %s", s.title())
	return s
}

// removeServer removes an item from the Servers submenu.
func (a *app) removeServer(s *server) {
	if s == nil {
		return
	}
	active := s.item.IsChecked()
	a.servers = slices.DeleteFunc(a.servers, func(x *server) bool { return x == s })
	a.serversMenu.Remove(s.item)
	if active && len(a.servers) > 0 {
		a.servers[0].item.SetChecked(true)
	}
	a.syncMenu()
	a.setStatus("Removed %s", s.title())
}

// removeLast removes the last item of the Servers submenu.
func (a *app) removeLast() {
	if len(a.servers) == 0 {
		a.setStatus("There is no server to remove")
		return
	}
	a.removeServer(a.servers[len(a.servers)-1])
}

// renameActive gives the active server, or the first, another name.
func (a *app) renameActive() {
	s := a.selected()
	if s == nil {
		a.setStatus("There is no server to rename")
		return
	}
	old := s.title()
	for range cities {
		if name := a.nextCityName(); name != s.name {
			s.name = name
			break
		}
	}
	s.item.SetLabel(s.title())
	a.setStatus("Renamed %s to %s", old, s.title())
}

// selected returns the server whose radio item is checked, or the first.
func (a *app) selected() *server {
	for _, s := range a.servers {
		if s.item.IsChecked() {
			return s
		}
	}
	if len(a.servers) > 0 {
		return a.servers[0]
	}
	return nil
}

// syncMenu shows the state of the items, and installs the menu again after
// items were added or removed: a tray shows the items it was given last.
func (a *app) syncMenu() {
	a.serversItem.SetLabel(fmt.Sprintf("Servers (%d)", len(a.servers)))
	a.renameItem.SetEnabled(len(a.servers) > 0)
	a.removeItem.SetEnabled(len(a.servers) > 0)
	if a.tray != nil {
		a.tray.SetMenu(a.menu)
	}
	if a.win != nil {
		a.win.Invalidate()
	}
}

// setStatus shows a line in the window and the tray menu.
func (a *app) setStatus(format string, args ...any) {
	a.status = fmt.Sprintf(format, args...)
	a.statusItem.SetLabel(a.status)
	if a.win != nil {
		a.win.Invalidate()
	}
}

// show brings the window to the front, closing only hides it.
func (a *app) show() {
	if a.win == nil {
		return
	}
	a.win.Show()
	a.win.Focus()
}

func (a *app) ready() {
	a.buildMenu()
	a.addServer(a.nextCityName())
	a.addServer(a.nextCityName())

	var err error
	a.tray, err = mygo.NewTray(mygo.TrayOptions{
		ID:             "tray-menu",
		Icon:           trayIcon(),
		IconIsTemplate: true,
		ToolTip:        "Tray Menu",
		Menu:           a.menu,
	})
	if err != nil {
		// Linux needs libayatana-appindicator3 for tray icons. The window
		// still shows the menu's state without one.
		log.Println("tray:", err)
	} else {
		mygo.App.OnWillQuit(func(*mygo.QuitEvent) { a.tray.Destroy() })
	}

	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:   "Tray Menu",
		Width:   620,
		Height:  520,
		Content: ui.View(a.view),
	})
	a.win.OnClose(func(e *mygo.CloseEvent) {
		// Closing hides the window: the app lives in the tray. Without a
		// tray there is nowhere to bring it back from, so closing quits.
		if a.tray != nil && !a.quitting {
			e.PreventDefault()
			a.win.Hide()
		}
	})
	if a.tray != nil {
		a.setStatus("Ready. Open the tray menu to see the same servers.")
	} else {
		a.setStatus("The tray icon is unavailable; see the log for why.")
	}
}

// trayIcon draws three menu rows, black on transparent for macOS to tint,
// blue elsewhere to show on light and dark taskbars alike.
func trayIcon() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	ink := color.NRGBA{R: 79, G: 124, B: 255, A: 255}
	if runtime.GOOS == "darwin" {
		ink = color.NRGBA{A: 255}
	}
	for row := range 3 {
		y := 8 + row*7
		draw.Draw(img, image.Rect(10, y, 27, y+2), image.NewUniform(ink), image.Point{}, draw.Src)
		draw.Draw(img, image.Rect(5, y-1, 9, y+3), image.NewUniform(ink), image.Point{}, draw.Src)
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

func main() {
	a := &app{}
	mygo.App.SetName("Tray Menu")
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { a.quitting = true })
	mygo.App.WhenReady(a.ready)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
