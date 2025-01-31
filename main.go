package main

import (
	"embed"
	_ "embed"
	"encoding/json"
	"log"
	"net/http"

	sse "github.com/r3labs/sse/v2"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

func watchChannels(
	bibleChannel chan *ShownVerse,
	songChannel chan *ShownCouplet,
	userChannel chan bool,
	server *sse.Server,
) {
	var lastVerse *ShownVerse = nil
	var lastCouplet *ShownCouplet = nil

	for {
		select {
		case <-userChannel:
			data, err := json.Marshal(map[string]any{
				"type":    "sync",
				"verse":   lastVerse,
				"couplet": lastCouplet,
			})
			if err != nil {
				log.Println("Error marshalling verse", err.Error())
				continue
			}
			server.Publish("main", &sse.Event{Data: data})
		case verse := <-bibleChannel:
			if verse == nil {
				lastVerse = nil
				data, err := json.Marshal(map[string]string{
					"type": "hide_verse",
				})
				if err != nil {
					log.Println("Error marshalling verse", err.Error())
					continue
				}
				server.Publish("main", &sse.Event{Data: data})
				continue
			} else {
				lastVerse = verse
				data, err := json.Marshal(map[string]any{
					"type":  "show_verse",
					"verse": verse,
				})
				if err != nil {
					log.Println("Error marshalling verse", err.Error())
					continue
				}

				server.Publish("main", &sse.Event{Data: data})
			}
		case couplet := <-songChannel:
			if couplet == nil {
				lastCouplet = nil
				data, err := json.Marshal(map[string]string{
					"type": "hide_couplet",
				})
				if err != nil {
					log.Println("Error marshalling verse", err.Error())
					continue
				}
				server.Publish("main", &sse.Event{Data: data})
				continue
			} else {
				lastCouplet = couplet
				data, err := json.Marshal(map[string]any{
					"type":    "show_couplet",
					"couplet": couplet,
				})
				if err != nil {
					log.Println("Error marshalling verse", err.Error())
					continue
				}
				server.Publish("main", &sse.Event{Data: data})
			}
		}
	}
}

func createSSE(bibleChannel chan *ShownVerse, songChannel chan *ShownCouplet) {
	server := sse.New()
	server.AutoReplay = false
	server.CreateStream("main")

	userChannel := make(chan bool)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("./reciever/dist")))
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		userChannel <- true
		go func() {
			<-r.Context().Done()
			log.Println("Client disconnected")
		}()

		server.ServeHTTP(w, r)
	})

	go func() {
		watchChannels(bibleChannel, songChannel, userChannel, server)
	}()

	http.ListenAndServe(":9093", mux)
}

func createChannels() (bibleChannel chan *ShownVerse, songChannel chan *ShownCouplet) {
	bibleChannel = make(chan *ShownVerse)
	songChannel = make(chan *ShownCouplet)

	return bibleChannel, songChannel
}

// main function serves as the application's entry point. It initializes the application, creates a window,
// and starts a goroutine that emits a time-based event every second. It subsequently runs the application and
// logs any error that might occur.
func main() {
	bibleChannel, songChannel := createChannels()
	dbHandler := DbHandler{verseChannel: bibleChannel, coupletChannel: songChannel}
	go createSSE(bibleChannel, songChannel)

	// Create a new Wails application by providing the necessary options.
	// Variables 'Name' and 'Description' are for application metadata.
	// 'Assets' configures the asset server with the 'FS' variable pointing to the frontend files.
	// 'Bind' is a list of Go struct instances. The frontend has access to the methods of these instances.
	// 'Mac' options tailor the application when running an macOS.
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

	app.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
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

	// Run the application. This blocks until the application has been exited.
	err := app.Run()
	// If an error occurred while running the application, log it and exit.
	if err != nil {
		log.Fatal(err)
	}
}
