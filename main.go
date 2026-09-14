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

	audioService := NewAudioService()

	app := application.New(application.Options{
		Name:        "versebearer",
		Description: "Show Bible verses and christian songs",
		Services: []application.Service{
			application.NewService(&dbHandler),
			application.NewService(audioService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	dbHandler.app = app
	dbHandler.keepAwakeFn = keepAwake
	audioService.app = app

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
		// EnableFileDrop — правка 2 (перетаскивание фонограмм из системы):
		// без него элементы с data-file-drop-target не порождают
		// WindowFilesDropped вовсе.
		EnableFileDrop: true,
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

	// Правка 2: файлы, брошенные на элемент с data-file-drop-target
	// (список плейлиста — PlaylistPanel.svelte), приходят сюда с
	// НАСТОЯЩИМИ путями (ctx.setDroppedFiles, webview_window.go:1404) — то,
	// что нужно ImportTrack, и то, чего у веб-инпута нет вообще. Go здесь
	// только пересылает пути фронту одним событием: какую именно фонограмму
	// импортировать и в какой плейлист добавить решает уже сама вкладка
	// «Звук» (activePlaylist живёт во фронтовом сторе, не здесь).
	//
	// ⚠️ Именно OnWindowEvent, НЕ RegisterHook: handleDragAndDropMessage
	// (webview_window.go:1404-1419) читает подписчиков броска файлов ТОЛЬКО
	// из w.eventListeners (что кладёт OnWindowEvent), а RegisterHook кладёт
	// в отдельную карту w.eventHooks, которую этот обработчик не смотрит —
	// с RegisterHook колбэк не вызывается никогда. Остальные подписки в этом
	// файле (WindowDidMove/WindowDidResize/WindowClosing) намеренно остаются
	// на RegisterHook — их обработчики читают именно eventHooks.
	mainWindow.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		files := e.Context().DroppedFiles()
		if len(files) == 0 {
			return
		}
		audioService.emit("audio_files_dropped", files)
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

	// Не log.Fatal: он завершает процесс через os.Exit, а тогда отложенное
	// закрытие поискового индекса не выполнится.
	if err := app.Run(); err != nil {
		log.Println("Application stopped with error", err.Error())
	}
}
