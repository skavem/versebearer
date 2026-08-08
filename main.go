package main

import (
	"embed"
	_ "embed"
	"log"
	"sync"

	"changeme/backend/inits"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	bibleChannel, songChannel, qrChannel, styleChannel := createChannels()
	dbHandler := DbHandler{
		qr:     qrChannel,
		styleB: styleChannel,
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
	go createSSE(bibleChannel, songChannel, qrChannel, styleChannel, inits.DB)

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

	// Индекс открывается после присваивания app: первичная сборка идёт в
	// горутине и рапортует о прогрессе событиями, а до этой строки emit
	// молча их глотает.
	if err := dbHandler.openSearchIndex(); err != nil {
		log.Println("Error opening search index", err.Error())
	}
	// Bleve держит сегменты и файловый замок: без закрытия каждый выход
	// оставляет индекс грязным.
	defer func() {
		if dbHandler.searchIdx == nil {
			return
		}
		if err := dbHandler.searchIdx.Close(); err != nil {
			log.Println("Error closing search index", err.Error())
		}
	}()

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
	emitCurrentScreen := func(_ *application.WindowEvent) {
		go func() {
			id := screenIDForWindow(mainWindow)
			if id == "" {
				return
			}
			lastScreenMu.Lock()
			if id == lastScreenID {
				lastScreenMu.Unlock()
				return
			}
			lastScreenID = id
			lastScreenMu.Unlock()
			dbHandler.emit("current_screen", id)
		}()
	}
	// Track the active screen on both move and resize. Maximizing/restoring or
	// snapping the window fires WindowDidResize (not always WindowDidMove), and
	// the screen can change without a plain move.
	mainWindow.RegisterHook(events.Common.WindowDidMove, emitCurrentScreen)
	mainWindow.RegisterHook(events.Common.WindowDidResize, emitCurrentScreen)

	mainWindow.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		mainID := mainWindow.ID()
		for _, w := range app.Window.GetAll() {
			if w.ID() == mainID {
				continue
			}
			w.Close()
		}
	})

	// Не log.Fatal: он завершает процесс через os.Exit, а тогда отложенное
	// закрытие поискового индекса не выполнится.
	if err := app.Run(); err != nil {
		log.Println("Application stopped with error", err.Error())
	}
}
