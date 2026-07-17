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

// outputStylePayload is the per-output slice of the sync event's "styles"
// map: {"<outputID>": {verse, couplet, backdrop}}.
type outputStylePayload struct {
	Verse    VisualStyle    `json:"verse"`
	Couplet  VisualStyle    `json:"couplet"`
	Backdrop map[string]any `json:"backdrop"`
}

// buildOutputStyles resolves every persisted Output's theme from the DB
// (source of truth — not an in-memory cache, since output/theme CRUD is a
// rare operator action, not a hot path) and renders its verse/couplet/
// backdrop styles. It also returns the first output's styles for the sync
// event's legacy top-level verseStyle/coupletStyle/backdrop fields, so a
// receiver connecting without ?out= keeps working unchanged.
func buildOutputStyles() (map[string]outputStylePayload, VisualStyle, VisualStyle, map[string]any) {
	var outputs []models.Output
	inits.DB.Order("id ASC").Find(&outputs)

	styles := map[string]outputStylePayload{}
	defVerse := DefaultVerseStyle
	defCouplet := DefaultCoupletStyle
	defBackdrop := backdropToMap(Backdrop{BgType: "none"})

	for i, o := range outputs {
		t := resolveOutputTheme(o)
		p := outputStylePayload{
			Verse:    styleFromTheme(t, "verse"),
			Couplet:  styleFromTheme(t, "couplet"),
			Backdrop: backdropToMap(backdropFromTheme(t)),
		}
		styles[strconv.FormatUint(uint64(o.ID), 10)] = p
		if i == 0 {
			defVerse = p.Verse
			defCouplet = p.Couplet
			defBackdrop = p.Backdrop
		}
	}

	return styles, defVerse, defCouplet, defBackdrop
}

// outputIdsForTheme returns the ids of every persisted Output whose resolved
// theme (see resolveOutputTheme) is themeId, scoping a style/backdrop change
// on that theme to just the outputs it affects.
func outputIdsForTheme(themeId uint) []uint {
	var outputs []models.Output
	inits.DB.Find(&outputs)
	ids := []uint{}
	for _, o := range outputs {
		if resolveOutputTheme(o).ID == themeId {
			ids = append(ids, o.ID)
		}
	}
	return ids
}

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
	var lastFonts []models.Font
	inits.DB.Find(&lastFonts)

	for {
		event := map[string]any{}
		select {
		case <-userChannel:
			styles, defVerse, defCouplet, defBackdrop := buildOutputStyles()
			event["type"] = "sync"
			event["verse"] = lastVerse
			event["couplet"] = lastCouplet
			event["qr"] = qr
			event["fonts"] = lastFonts
			event["styles"] = styles
			event["verseStyle"] = defVerse
			event["coupletStyle"] = defCouplet
			event["backdrop"] = defBackdrop
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
				if styleEvt.ThemeId != 0 {
					event["outputIds"] = outputIdsForTheme(styleEvt.ThemeId)
				}
			} else if styleEvt.Type == "backdrop_update" {
				event["style"] = styleEvt.Style
				if styleEvt.ThemeId != 0 {
					event["outputIds"] = outputIdsForTheme(styleEvt.ThemeId)
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

//go:embed reciever/dist
var recAssets embed.FS

func createSSE(
	bibleChannel chan *ShownVerse,
	songChannel chan *ShownCouplet,
	qrChannel chan *bool,
	styleChannel chan *StyleEvent,
	db *gorm.DB,
) {
	// .env is a dev convenience only (DEV=true serves the receiver from disk
	// instead of the embedded FS). Production builds ship without it, so a
	// missing file is fine — fall back to the real process environment.
	_ = godotenv.Load()
	isDev := os.Getenv("DEV")

	server := sse.New()
	server.AutoReplay = false

	userChannel := make(chan bool)

	// OnSubscribe must be assigned before CreateStream — CreateStream snapshots
	// it into the stream at creation time, so setting it afterward is a no-op.
	// It fires from stream.addSubscriber *after* the subscriber is already
	// registered on the stream (registration happens synchronously; the
	// callback itself runs in its own goroutine), so triggering the sync build
	// here is guaranteed to reach this connection. That fixes a latent race in
	// the old approach: userChannel<-true used to be sent before
	// server.ServeHTTP(w, r) even ran, so a freshly opened window could miss
	// its initial sync if it wasn't registered yet by the time it was built.
	server.OnSubscribe = func(streamID string, sub *sse.Subscriber) {
		// out is parsed here so per-output filtering could hook in later; today
		// the sync payload already carries every output's style (see
		// buildOutputStyles), so all subscribers receive identical content and
		// filter client-side by their own ?out=.
		_ = sub.URL.Query().Get("out")
		userChannel <- true
	}
	server.CreateStream("main")

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

	mux.HandleFunc("/image/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/image/")
		idStr := path
		if i := strings.IndexByte(path, '.'); i >= 0 {
			idStr = path[:i]
		}
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		var im models.Image
		if err := db.First(&im, uint(id)).Error; err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", im.MimeType)
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Write(im.Data)
	})

	mux.Handle("/", fsServer)
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
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
