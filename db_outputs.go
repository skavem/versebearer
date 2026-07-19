package main

import (
	"changeme/backend/inits"
	"changeme/backend/models"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"log"
	"strconv"
	"time"
)

// screenIDForWindow resolves which screen a window sits on by its center
// point rather than its bounding rectangle. On Windows a maximized window's
// rect (GetWindowRect) is inflated by the invisible resize border (~8px), so
// it spills a few pixels onto an adjacent monitor. The bounds-based
// GetScreen() then ties at distance 0 between both monitors and returns
// whichever is listed first (usually the primary), so a window maximized on
// the secondary monitor can be reported as being on the primary. The center
// point is unambiguously inside a single monitor.
func screenIDForWindow(w application.Window) string {
	b := w.Bounds()
	center := application.Point{
		X: b.X + b.Width/2,
		Y: b.Y + b.Height/2,
	}
	scr := application.ScreenNearestDipPoint(center)
	if scr == nil {
		return ""
	}
	return scr.ID
}

func (g *DbHandler) GetCurrentScreenID() string {
	if g.app == nil {
		return ""
	}
	w, ok := g.app.Window.GetByName(mainWindowName)
	if !ok {
		return ""
	}
	return screenIDForWindow(w)
}

// projectorWindowOptions configures a projector window opened via
// openProjectorWindow. "display" mode (the original, still used by
// ShowScreen and Output.Mode=="display") is frameless, always-on-top, sized
// to fill a monitor exactly; "window" mode (models.Output.Mode=="window") is
// a normal window at an operator-chosen position/size with independently
// toggleable frame/always-on-top.
type projectorWindowOptions struct {
	X, Y, Width, Height int
	Name, URL           string
	Transparent         bool
	Frameless           bool
	AlwaysOnTop         bool
	Resizable           bool
	IgnoreMouseEvents   bool
}

// openProjectorWindow creates the window that hosts the receiver page at
// url and wires up the shared "window closed" notification. Shared by
// ShowScreen (legacy monitor-only flow) and StartOutput (per-output flow).
func (g *DbHandler) openProjectorWindow(o projectorWindowOptions) application.Window {
	app := application.Get()

	opts := application.WebviewWindowOptions{
		Name:          o.Name,
		Title:         "VerseBearer - screen",
		Frameless:     o.Frameless,
		X:             o.X,
		Y:             o.Y,
		Width:         o.Width,
		Height:        o.Height,
		AlwaysOnTop:   o.AlwaysOnTop,
		DisableResize: !o.Resizable,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		KeyBindings: map[string]func(application.Window){
			"Escape": func(win application.Window) { win.Close() },
		},
	}

	if o.Transparent {
		// Overlay mode: a see-through window so the projected verse/couplet
		// text floats over whatever else is on that monitor. The receiver
		// reads ?transparent=1 and drops its opaque page background.
		opts.BackgroundType = application.BackgroundTypeTransparent
		opts.BackgroundColour = application.NewRGBA(0, 0, 0, 0)
	}
	opts.IgnoreMouseEvents = o.IgnoreMouseEvents
	opts.URL = o.URL

	w := app.Window.NewWithOptions(opts)

	w.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		g.emit("screen_closed", o.Name)
	})

	return w
}

func (g *DbHandler) ShowScreen(x, y, sizeX, sizeY float32, name string, transparent bool) {
	url := "http://localhost:9093"
	if transparent {
		url += "?transparent=1"
	}
	g.openProjectorWindow(projectorWindowOptions{
		X: int(x), Y: int(y), Width: int(sizeX), Height: int(sizeY),
		Name: name, URL: url, Transparent: transparent,
		Frameless: true, AlwaysOnTop: true, Resizable: true,
		// Mouse events pass through to the app underneath — only meaningful
		// (and only enabled) for the see-through transparent overlay.
		IgnoreMouseEvents: transparent,
	})
}

func (g *DbHandler) CloseScreen(name string) {
	app := application.Get()

	s, ok := app.Window.GetByName(name)
	if !ok {
		return
	}
	s.Close()
}

// outputWindowName is the Wails window name for a persisted Output's
// projector window, e.g. "out-3".
func outputWindowName(id uint) string {
	return "out-" + strconv.FormatUint(uint64(id), 10)
}

// StartOutput opens (or reopens) the projector window for a persisted Output,
// loading the receiver with ?out=<id> so it filters style/backdrop events
// down to just this output (see sse.go/App.svelte). x,y,w,h are the bounds of
// a reference monitor supplied by the frontend: in "display" mode the window
// fills that monitor exactly; in "window" mode they're only used to center
// the window the first time it's ever opened (see models.Output.WinPlaced) —
// afterwards its own WinX/WinY take over.
func (g *DbHandler) StartOutput(idF, x, y, w, h float32) {
	id := uint(idF)
	var o models.Output
	if err := inits.DB.First(&o, id).Error; err != nil {
		log.Println("StartOutput: output not found", err)
		return
	}
	transparentFlag := "0"
	if o.Transparent {
		transparentFlag = "1"
	}
	idStr := strconv.FormatUint(uint64(id), 10)
	url := "http://localhost:9093?out=" + idStr + "&transparent=" + transparentFlag
	name := outputWindowName(id)

	if o.Mode == "window" {
		winW, winH := o.WinWidth, o.WinHeight
		if winW == 0 {
			winW = 1280
		}
		if winH == 0 {
			winH = 720
		}
		winX, winY := o.WinX, o.WinY
		if !o.WinPlaced {
			// Never been placed — center it on the reference monitor instead
			// of using the stale/zero WinX/WinY.
			winX = int(x) + (int(w)-winW)/2
			winY = int(y) + (int(h)-winH)/2
		}
		win := g.openProjectorWindow(projectorWindowOptions{
			X: winX, Y: winY, Width: winW, Height: winH,
			Name: name, URL: url, Transparent: o.Transparent,
			Frameless: o.Frameless, AlwaysOnTop: o.AlwaysOnTop, Resizable: true,
			// A window-mode output is a normal interactive window even when
			// transparent (e.g. for OBS capture) — display mode is the only
			// one that click-throughs.
			IgnoreMouseEvents: false,
		})
		g.watchWindowGeometry(id, win)
		return
	}

	g.openProjectorWindow(projectorWindowOptions{
		X: int(x), Y: int(y), Width: int(w), Height: int(h),
		Name: name, URL: url, Transparent: o.Transparent,
		Frameless: true, AlwaysOnTop: true, Resizable: true,
		IgnoreMouseEvents: o.Transparent,
	})
}

// StopOutput closes the projector window for a persisted Output, if open.
func (g *DbHandler) StopOutput(idF float32) {
	id := uint(idF)
	g.cancelWindowGeometrySave(id)
	g.CloseScreen(outputWindowName(id))
}

// watchWindowGeometry keeps a window-mode output's WinX/WinY/WinWidth/
// WinHeight in sync with the operator dragging/resizing its projector
// window, so reopening the output restores where they left it (WinPlaced
// flips true on the first save). Persistence is debounced — see
// scheduleWindowGeometrySave — and cancelled once the window closes.
func (g *DbHandler) watchWindowGeometry(id uint, w application.Window) {
	save := func(_ *application.WindowEvent) { g.scheduleWindowGeometrySave(id, w) }
	w.RegisterHook(events.Common.WindowDidMove, save)
	w.RegisterHook(events.Common.WindowDidResize, save)
	w.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		g.cancelWindowGeometrySave(id)
	})
}

// scheduleWindowGeometrySave debounces (~400ms) writing w's current bounds to
// Output id, so a drag or resize doesn't issue a SQLite write per pixel/frame
// — only once movement settles.
func (g *DbHandler) scheduleWindowGeometrySave(id uint, w application.Window) {
	const debounce = 400 * time.Millisecond

	g.winSaveMu.Lock()
	defer g.winSaveMu.Unlock()
	if g.winSaveTimers == nil {
		g.winSaveTimers = map[uint]*time.Timer{}
	}
	if t, ok := g.winSaveTimers[id]; ok {
		t.Stop()
	}
	g.winSaveTimers[id] = time.AfterFunc(debounce, func() {
		b := w.Bounds()
		if err := inits.DB.Model(&models.Output{}).Where("id = ?", id).Updates(map[string]any{
			"win_x": b.X, "win_y": b.Y, "win_width": b.Width, "win_height": b.Height,
			"win_placed": true,
		}).Error; err != nil {
			log.Println("scheduleWindowGeometrySave: error saving geometry", err)
		}
	})
}

// cancelWindowGeometrySave stops a pending debounced geometry save, if any —
// called when the output's window closes so a save doesn't fire afterwards
// against a now-invalid window handle.
func (g *DbHandler) cancelWindowGeometrySave(id uint) {
	g.winSaveMu.Lock()
	defer g.winSaveMu.Unlock()
	if t, ok := g.winSaveTimers[id]; ok {
		t.Stop()
		delete(g.winSaveTimers, id)
	}
}

// --- Output CRUD ---

func (g *DbHandler) ListOutputs() []models.Output {
	var outputs []models.Output
	if err := inits.DB.Order("id ASC").Find(&outputs).Error; err != nil {
		log.Println("ListOutputs: error", err)
		return nil
	}
	return outputs
}

// CreateOutput adds a new output pointing at the default theme.
func (g *DbHandler) CreateOutput(name string) *models.Output {
	var themeId *uint
	if def, err := loadDefaultTheme(); err == nil {
		id := def.ID
		themeId = &id
	} else {
		log.Println("CreateOutput: error loading default theme", err)
	}
	o := models.Output{
		Name: name, ThemeId: themeId, Transparent: false, ScreenID: "",
		Mode: "display", WinWidth: 1280, WinHeight: 720,
	}
	if err := inits.DB.Create(&o).Error; err != nil {
		log.Println("CreateOutput: error creating output", err)
		return nil
	}
	return &o
}

// updateOutputField persists a single column on one output and returns the
// refreshed list — the shared shape of the simple output setters below.
func (g *DbHandler) updateOutputField(idF float32, col string, val any) []models.Output {
	if err := inits.DB.Model(&models.Output{}).Where("id = ?", uint(idF)).Update(col, val).Error; err != nil {
		log.Printf("updateOutputField(%s): error: %v", col, err)
	}
	return g.ListOutputs()
}

func (g *DbHandler) RenameOutput(idF float32, name string) []models.Output {
	return g.updateOutputField(idF, "name", name)
}

func (g *DbHandler) DeleteOutput(idF float32) []models.Output {
	if err := inits.DB.Delete(&models.Output{}, uint(idF)).Error; err != nil {
		log.Println("DeleteOutput: error deleting", err)
	}
	return g.ListOutputs()
}

func (g *DbHandler) SetOutputScreen(idF float32, screenId string) []models.Output {
	return g.updateOutputField(idF, "screen_id", screenId)
}

func (g *DbHandler) SetOutputTransparent(idF float32, v bool) []models.Output {
	return g.updateOutputField(idF, "transparent", v)
}

// SetOutputMode persists an output's presentation mode ("display" | "window").
// Switching mode changes window-creation options that can't all be applied
// live (initial size/position, click-through) — the frontend stops+starts
// the output afterwards if it's currently projecting, so this only persists.
func (g *DbHandler) SetOutputMode(idF float32, mode string) []models.Output {
	return g.updateOutputField(idF, "mode", mode)
}

// SetOutputWindowSize persists a window-mode output's size and, if its
// projector window is currently open, resizes it live.
func (g *DbHandler) SetOutputWindowSize(idF, wF, hF float32) []models.Output {
	id := uint(idF)
	width, height := int(wF), int(hF)
	if err := inits.DB.Model(&models.Output{}).Where("id = ?", id).Updates(map[string]any{
		"win_width": width, "win_height": height,
	}).Error; err != nil {
		log.Println("SetOutputWindowSize: error", err)
	}
	if g.app != nil {
		if w, ok := g.app.Window.GetByName(outputWindowName(id)); ok {
			w.SetSize(width, height)
		}
	}
	return g.ListOutputs()
}

// SetOutputFrameless persists a window-mode output's frame toggle and, if its
// projector window is currently open, applies it live via SetFrameless.
func (g *DbHandler) SetOutputFrameless(idF float32, v bool) []models.Output {
	id := uint(idF)
	if err := inits.DB.Model(&models.Output{}).Where("id = ?", id).Update("frameless", v).Error; err != nil {
		log.Println("SetOutputFrameless: error", err)
	}
	if g.app != nil {
		if w, ok := g.app.Window.GetByName(outputWindowName(id)); ok {
			w.SetFrameless(v)
		}
	}
	return g.ListOutputs()
}

// SetOutputAlwaysOnTop persists a window-mode output's always-on-top toggle
// and, if its projector window is currently open, applies it live via
// SetAlwaysOnTop.
func (g *DbHandler) SetOutputAlwaysOnTop(idF float32, v bool) []models.Output {
	id := uint(idF)
	if err := inits.DB.Model(&models.Output{}).Where("id = ?", id).Update("always_on_top", v).Error; err != nil {
		log.Println("SetOutputAlwaysOnTop: error", err)
	}
	if g.app != nil {
		if w, ok := g.app.Window.GetByName(outputWindowName(id)); ok {
			w.SetAlwaysOnTop(v)
		}
	}
	return g.ListOutputs()
}

// SetOutputTheme repoints an output at a different theme and immediately
// broadcasts that theme's full style/backdrop so the output's live
// projection (if open) updates right away — a full re-render, not a patch,
// since the output could be coming from an entirely different theme.
func (g *DbHandler) SetOutputTheme(idF float32, themeIdF float32) []models.Output {
	id := uint(idF)
	themeId := uint(themeIdF)
	if err := inits.DB.Model(&models.Output{}).Where("id = ?", id).Update("theme_id", themeId).Error; err != nil {
		log.Println("SetOutputTheme: error", err)
		return g.ListOutputs()
	}
	var t models.Theme
	if err := inits.DB.First(&t, themeId).Error; err == nil {
		g.broadcastTheme(t)
	} else {
		log.Println("SetOutputTheme: new theme not found", err)
	}
	return g.ListOutputs()
}
