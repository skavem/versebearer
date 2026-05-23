package main

import (
	"embed"
	_ "embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"changeme/backend/inits"
	"changeme/backend/models"

	"github.com/joho/godotenv"
	sse "github.com/r3labs/sse/v2"
	"gorm.io/gorm"
)

func watchChannels(
	bibleChannel chan *ShownVerse,
	songChannel chan *ShownCouplet,
	userChannel chan bool,
	qrChannel chan *bool,
	styleChannel chan *StyleEvent,
	server *sse.Server,
) {
	var lastVerse *ShownVerse = nil
	var lastCouplet *ShownCouplet = nil
	var qr bool = false
	var lastVerseStyle VisualStyle
	var lastCoupletStyle VisualStyle
	var lastFonts []models.Font

	// Initialize style cache from DB
	gs := models.GlobalState{}
	if err := inits.DB.First(&gs, 1).Error; err == nil {
		lastVerseStyle = visualStyleFromGS(gs, "verse")
		lastCoupletStyle = visualStyleFromGS(gs, "couplet")
	} else {
		lastVerseStyle = DefaultVerseStyle
		lastCoupletStyle = DefaultCoupletStyle
	}
	inits.DB.Find(&lastFonts)

	for {
		event := map[string]any{}
		select {
		case <-userChannel:
			event["type"] = "sync"
			event["verse"] = lastVerse
			event["couplet"] = lastCouplet
			event["qr"] = qr
			event["verseStyle"] = lastVerseStyle
			event["coupletStyle"] = lastCoupletStyle
			event["fonts"] = lastFonts
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
		case styleEvt := <-styleChannel:
			if styleEvt == nil {
				continue
			}
			event["type"] = styleEvt.Type
			if styleEvt.Type == "style_update" {
				event["target"] = styleEvt.Target
				event["style"] = styleEvt.Style
				if styleEvt.Target == "verse" {
					mergeStyle(&lastVerseStyle, styleEvt.Style)
				} else if styleEvt.Target == "couplet" {
					mergeStyle(&lastCoupletStyle, styleEvt.Style)
				}
			} else if styleEvt.Type == "fonts_changed" {
				event["fonts"] = styleEvt.Fonts
				lastFonts = styleEvt.Fonts
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

func mergeStyle(s *VisualStyle, m map[string]any) {
	if v, ok := m["bgColor"].(string); ok {
		s.BgColor = v
	}
	if v, ok := m["bgOpacity"].(float64); ok {
		s.BgOpacity = v
	}
	if v, ok := m["textColor"].(string); ok {
		s.TextColor = v
	}
	if v, ok := m["fontId"]; ok {
		switch fv := v.(type) {
		case *uint:
			s.FontId = fv
		case nil:
			s.FontId = nil
		}
	}
	if v, ok := m["borderColor"].(string); ok {
		s.BorderColor = v
	}
	if v, ok := m["borderWidth"].(int); ok {
		s.BorderWidth = v
	}
	if v, ok := m["borderRadius"].(int); ok {
		s.BorderRadius = v
	}
	if v, ok := m["borderStyle"].(string); ok {
		s.BorderStyle = v
	}
	if v, ok := m["padding"].(int); ok {
		s.Padding = v
	}
	if v, ok := m["textShadow"].(string); ok {
		s.TextShadow = v
	}
}

//go:embed reciever/dist
var recAssets embed.FS

func createSSE(
	bibleChannel chan *ShownVerse,
	songChannel chan *ShownCouplet,
	qrChannel chan *bool,
	styleChannel chan *StyleEvent,
	db *gorm.DB,
) {
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
	fsServer := http.FileServer(fsys)

	mux.HandleFunc("/font/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/font/")
		idStr := path
		if i := strings.IndexByte(path, '.'); i >= 0 {
			idStr = path[:i]
		}
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		var f models.Font
		if err := db.First(&f, uint(id)).Error; err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", f.MimeType)
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Write(f.Data)
	})

	mux.Handle("/", fsServer)
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		userChannel <- true
		go func() {
			<-r.Context().Done()
			log.Println("Client disconnected")
		}()

		server.ServeHTTP(w, r)
	})

	go func() {
		watchChannels(bibleChannel, songChannel, userChannel, qrChannel, styleChannel, server)
	}()

	http.ListenAndServe(":9093", mux)
}

func createChannels() (bibleChannel chan *ShownVerse, songChannel chan *ShownCouplet, qrChannel chan *bool, styleChannel chan *StyleEvent) {
	bibleChannel = make(chan *ShownVerse)
	songChannel = make(chan *ShownCouplet)
	qrChannel = make(chan *bool)
	styleChannel = make(chan *StyleEvent)

	return bibleChannel, songChannel, qrChannel, styleChannel
}
