package main

import (
	"embed"
	_ "embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	sse "github.com/r3labs/sse/v2"
)

func watchChannels(
	bibleChannel chan *ShownVerse,
	songChannel chan *ShownCouplet,
	userChannel chan bool,
	qrChannel chan *bool,
	server *sse.Server,
) {
	var lastVerse *ShownVerse = nil
	var lastCouplet *ShownCouplet = nil
	var qr bool = false

	for {
		event := map[string]any{}
		select {
		case <-userChannel:
			event["type"] = "sync"
			event["verse"] = lastVerse
			event["couplet"] = lastCouplet
			event["qr"] = qr
		case verse := <-bibleChannel:
			event["type"] = "hide_verse"
			lastVerse = nil
			if verse != nil {
				event["verse"] = verse
				event["type"] = "show_verse"
				lastVerse = verse
			}
		case couplet := <-songChannel:
			event["type"] = "hide_couplet"
			lastCouplet = nil
			if couplet != nil {
				event["couplet"] = couplet
				event["type"] = "show_couplet"
				lastCouplet = couplet
			}
		case curQr := <-qrChannel:
			qr = *curQr
			event["type"] = "hide_qr"
			if qr {
				event["type"] = "show_qr"
			}
		}
		data, err := json.Marshal(event)
		if err != nil {
			log.Println("Error marshalling event", err.Error())
			continue
		}
		server.Publish("main", &sse.Event{Data: data})
	}
}

//go:embed reciever/dist
var recAssets embed.FS

func createSSE(bibleChannel chan *ShownVerse, songChannel chan *ShownCouplet, qrChannel chan *bool) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file", err.Error())
	}
	isDev := os.Getenv("DEV")

	server := sse.New()
	server.AutoReplay = false
	server.CreateStream("main")

	userChannel := make(chan bool)
	mux := http.NewServeMux()

	dist, err := fs.Sub(recAssets, "reciever/dist")
	if err != nil {
		log.Fatal(err)
	}
	var fsys http.FileSystem
	if isDev == "true" {
		fsys = http.Dir("./reciever/dist")
	} else {
		fsys = http.FS(dist)
	}
	fs := http.FileServer(fsys)

	mux.Handle("/", fs)
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		userChannel <- true
		go func() {
			<-r.Context().Done()
			log.Println("Client disconnected")
		}()

		server.ServeHTTP(w, r)
	})

	go func() {
		watchChannels(bibleChannel, songChannel, userChannel, qrChannel, server)
	}()

	http.ListenAndServe(":9093", mux)
}

func createChannels() (bibleChannel chan *ShownVerse, songChannel chan *ShownCouplet, qrChannel chan *bool) {
	bibleChannel = make(chan *ShownVerse)
	songChannel = make(chan *ShownCouplet)
	qrChannel = make(chan *bool)

	return bibleChannel, songChannel, qrChannel
}

