package main

import (
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
)

// buildMenu membuat menu native macOS. Callback hanya meng-emit event Wails;
// frontend yang memutuskan aksi.
func buildMenu(b *Bridge) *menu.Menu {
	emitPlain := func(name string) menu.Callback {
		return func(_ *menu.CallbackData) { b.emit(name) }
	}
	emitZoom := func(action string) menu.Callback {
		return func(_ *menu.CallbackData) {
			b.emit(EventMenuZoom, map[string]string{"action": action})
		}
	}

	m := menu.NewMenu()
	m.Append(menu.AppMenu())

	file := menu.NewMenu()
	file.Append(menu.Text("Open Folder...", keys.CmdOrCtrl("o"), emitPlain(EventMenuOpenFolder)))
	file.Append(menu.Text("New Workspace from Folder...",
		keys.Combo("o", keys.CmdOrCtrlKey, keys.ShiftKey), emitPlain(EventMenuNewWorkspaceFromFolder)))
	m.Append(menu.SubMenu("File", file))

	m.Append(menu.EditMenu())

	view := menu.NewMenu()
	view.Append(menu.Text("Zoom In", keys.CmdOrCtrl("="), emitZoom("in")))
	view.Append(menu.Text("Zoom Out", keys.CmdOrCtrl("-"), emitZoom("out")))
	view.Append(menu.Text("Actual Size", keys.CmdOrCtrl("0"), emitZoom("reset")))
	view.Append(menu.Separator())
	view.Append(menu.Text("Toggle Left Sidebar", keys.CmdOrCtrl("b"), emitPlain(EventMenuToggleSidebarLeft)))
	view.Append(menu.Text("Toggle Right Sidebar",
		keys.Combo("b", keys.CmdOrCtrlKey, keys.OptionOrAltKey), emitPlain(EventMenuToggleSidebarRight)))
	m.Append(menu.SubMenu("View", view))

	m.Append(menu.WindowMenu())
	return m
}
