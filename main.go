package main

import (
	"embed"
	_ "embed"
	"log"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	bibleChannel, songChannel, qrChannel := createChannels()
	dbHandler := DbHandler{
		qr: qrChannel,
	}
	dbHandler.verseB = &broadcaster[ShownVerse]{
		ch:      bibleChannel,
		showEvt: "show_verse",
		hideEvt: "hide_verse",
		emit:    dbHandler.emit,
	}
	dbHandler.coupletB = &broadcaster[ShownCouplet]{
		ch:      songChannel,
		showEvt: "show_couplet",
		hideEvt: "hide_couplet",
		emit:    dbHandler.emit,
	}
	go createSSE(bibleChannel, songChannel, qrChannel)

	app := application.New(application.Options{
		Name:        "versebearer",
		Description: "Show Bible verses and christian songs",
		Services: []application.Service{
			application.NewService(&dbHandler),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	dbHandler.app = app

	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      mainWindowName,
		Title:     "VerseBearer",
		MinWidth:  900,
		Width:     900,
		MinHeight: 700,
		Height:    700,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(100, 100, 100),
		URL:              "/",
	})

	var (
		lastScreenMu sync.Mutex
		lastScreenID string
	)
	mainWindow.RegisterHook(events.Common.WindowDidMove, func(_ *application.WindowEvent) {
		go func() {
			scr, err := mainWindow.GetScreen()
			if err != nil || scr == nil {
				return
			}
			lastScreenMu.Lock()
			if scr.ID == lastScreenID {
				lastScreenMu.Unlock()
				return
			}
			lastScreenID = scr.ID
			lastScreenMu.Unlock()
			dbHandler.emit("current_screen", scr.ID)
		}()
	})

	mainWindow.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		mainID := mainWindow.ID()
		for _, w := range app.Window.GetAll() {
			if w.ID() == mainID {
				continue
			}
			w.Close()
		}
	})

	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
