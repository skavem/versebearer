package main

import (
	"embed"
	_ "embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
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

	app.Window.NewWithOptions(application.WebviewWindowOptions{
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

	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
