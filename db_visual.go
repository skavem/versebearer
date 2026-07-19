package main

import (
	"changeme/backend/inits"
	"changeme/backend/models"
	"gorm.io/gorm"
	"log"
)

// --- Visual / style handlers ---

// normalizeBgType maps the empty zero-value (legacy themes predating backdrops)
// to "none" so the rest of the pipeline always sees a valid value.
func normalizeBgType(t string) string {
	if t == "" {
		return "none"
	}
	return t
}

func styleFromTheme(t models.Theme, target string) VisualStyle {
	if target == "verse" {
		return VisualStyle{
			BgColor:      t.VerseBgColor,
			BgOpacity:    t.VerseBgOpacity,
			TextColor:    t.VerseTextColor,
			FontId:       t.VerseFontId,
			BorderColor:  t.VerseBorderColor,
			BorderWidth:  t.VerseBorderWidth,
			BorderRadius: t.VerseBorderRadius,
			BorderStyle:  t.VerseBorderStyle,
			Padding:      t.VersePadding,
			Margin:       t.VerseMargin,
			TextShadow:   t.VerseTextShadow,
		}
	}
	return VisualStyle{
		BgColor:      t.CoupletBgColor,
		BgOpacity:    t.CoupletBgOpacity,
		TextColor:    t.CoupletTextColor,
		FontId:       t.CoupletFontId,
		BorderColor:  t.CoupletBorderColor,
		BorderWidth:  t.CoupletBorderWidth,
		BorderRadius: t.CoupletBorderRadius,
		BorderStyle:  t.CoupletBorderStyle,
		Padding:      t.CoupletPadding,
		Margin:       t.CoupletMargin,
		TextShadow:   t.CoupletTextShadow,
	}
}

// backdropFromTheme extracts the theme's single always-on backdrop.
func backdropFromTheme(t models.Theme) Backdrop {
	return Backdrop{
		BgType:     normalizeBgType(t.BgType),
		BgGradient: t.BgGradient,
		BgImageId:  t.BgImageId,
	}
}

func backdropToMap(b Backdrop) map[string]any {
	return map[string]any{
		"bgType": b.BgType, "bgGradient": b.BgGradient, "bgImageId": b.BgImageId,
	}
}

// styleToMap renders a VisualStyle into the loosely-typed payload the receiver
// merges (see mergeStyle in sse.go). Central so every broadcast stays in sync.
func styleToMap(s VisualStyle) map[string]any {
	return map[string]any{
		"bgColor": s.BgColor, "bgOpacity": s.BgOpacity,
		"textColor": s.TextColor, "fontId": s.FontId,
		"borderColor": s.BorderColor, "borderWidth": s.BorderWidth,
		"borderRadius": s.BorderRadius, "borderStyle": s.BorderStyle,
		"padding": s.Padding, "margin": s.Margin, "textShadow": s.TextShadow,
	}
}

// loadActiveTheme returns the theme GlobalState.ActiveThemeId points at,
// falling back to the default theme if the pointer is unset or dangling.
func loadActiveTheme() (models.Theme, error) {
	gs := models.GlobalState{}
	if err := inits.DB.First(&gs, 1).Error; err != nil {
		return models.Theme{}, err
	}
	var t models.Theme
	if gs.ActiveThemeId != nil {
		if err := inits.DB.First(&t, *gs.ActiveThemeId).Error; err == nil {
			return t, nil
		}
	}
	if err := inits.DB.Where("is_default = ?", true).First(&t).Error; err != nil {
		return models.Theme{}, err
	}
	return t, nil
}

// loadDefaultTheme returns the theme with IsDefault=true, falling back to the
// first theme (by id) if none is marked default (should not normally happen —
// the default theme cannot be deleted, see DeleteTheme).
func loadDefaultTheme() (models.Theme, error) {
	var t models.Theme
	if err := inits.DB.Where("is_default = ?", true).First(&t).Error; err == nil {
		return t, nil
	}
	if err := inits.DB.Order("id ASC").First(&t).Error; err != nil {
		return models.Theme{}, err
	}
	return t, nil
}

// resolveOutputTheme returns the theme that renders Output o: its own
// ThemeId if set and still valid, otherwise the default theme. The Output
// row is the source of truth for this resolution, resolved fresh from the DB
// each time — operator edits (CRUD on outputs/themes) are rare, not a hot
// path, so no in-memory registry is kept in sync.
func resolveOutputTheme(o models.Output) models.Theme {
	if o.ThemeId != nil {
		var t models.Theme
		if err := inits.DB.First(&t, *o.ThemeId).Error; err == nil {
			return t
		}
	}
	t, _ := loadDefaultTheme()
	return t
}

// broadcastTheme pushes both verse and couplet styles of t to the receiver
// using the existing style_update mechanism (receiver is unchanged). ThemeId
// lets watchChannels scope the change to the outputs currently resolving to
// t (see outputIdsForTheme in sse.go).
func (g *DbHandler) broadcastTheme(t models.Theme) {
	g.styleB <- &StyleEvent{Type: "style_update", Target: "verse", Style: styleToMap(styleFromTheme(t, "verse")), ThemeId: t.ID}
	g.styleB <- &StyleEvent{Type: "style_update", Target: "couplet", Style: styleToMap(styleFromTheme(t, "couplet")), ThemeId: t.ID}
	g.styleB <- &StyleEvent{Type: "backdrop_update", Style: backdropToMap(backdropFromTheme(t)), ThemeId: t.ID}
}

func (g *DbHandler) GetVisualSettings() VisualSettings {
	t, err := loadActiveTheme()
	if err != nil {
		log.Println("GetVisualSettings: error loading active theme", err)
	}
	// Data blobs are served separately via /font/{id} and /image/{id}; omit them
	// here so opening the Визуал tab doesn't read every font/image into memory.
	var fonts []models.Font
	inits.DB.Omit("Data").Find(&fonts)
	var images []models.Image
	inits.DB.Omit("Data").Find(&images)
	return VisualSettings{
		VerseStyle:   styleFromTheme(t, "verse"),
		CoupletStyle: styleFromTheme(t, "couplet"),
		Backdrop:     backdropFromTheme(t),
		Fonts:        fonts,
		Images:       images,
	}
}

// UpdateBackdrop writes the given fields into the active theme's single backdrop
// and broadcasts them (always-on, independent of verse/couplet visibility).
func (g *DbHandler) UpdateBackdrop(input BackdropInput) Backdrop {
	t, err := loadActiveTheme()
	if err != nil {
		log.Println("UpdateBackdrop: error loading active theme", err)
		return backdropFromTheme(t)
	}
	updates := map[string]any{}
	if input.BgType != nil {
		t.BgType = *input.BgType
		updates["bg_type"] = *input.BgType
	}
	if input.BgGradient != nil {
		t.BgGradient = *input.BgGradient
		updates["bg_gradient"] = *input.BgGradient
	}
	if input.BgImageId != nil {
		t.BgImageId = input.BgImageId
		updates["bg_image_id"] = input.BgImageId
	}
	if len(updates) > 0 {
		if err := inits.DB.Model(&t).Updates(updates).Error; err != nil {
			log.Println("UpdateBackdrop: error saving", err)
		}
	}
	b := backdropFromTheme(t)
	g.styleB <- &StyleEvent{Type: "backdrop_update", Style: backdropToMap(b), ThemeId: t.ID}
	return b
}

// ResetBackdrop turns the active theme's backdrop off.
func (g *DbHandler) ResetBackdrop() Backdrop {
	t, err := loadActiveTheme()
	if err != nil {
		log.Println("ResetBackdrop: error loading active theme", err)
		return Backdrop{BgType: "none"}
	}
	if err := inits.DB.Model(&t).Updates(map[string]any{
		"bg_type":     "none",
		"bg_gradient": "",
		"bg_image_id": nil,
	}).Error; err != nil {
		log.Println("ResetBackdrop: error saving", err)
	}
	b := Backdrop{BgType: "none"}
	g.styleB <- &StyleEvent{Type: "backdrop_update", Style: backdropToMap(b), ThemeId: t.ID}
	return b
}

// styleInputToUpdates turns the optional style fields into a column→value map
// for the given target ("verse"/"couplet"), which selects the column prefix.
func styleInputToUpdates(target string, input StyleInput) map[string]any {
	updates := map[string]any{}
	set := func(suffix string, v any) { updates[target+"_"+suffix] = v }
	if input.BgColor != nil {
		set("bg_color", *input.BgColor)
	}
	if input.BgOpacity != nil {
		set("bg_opacity", *input.BgOpacity)
	}
	if input.TextColor != nil {
		set("text_color", *input.TextColor)
	}
	if input.FontId != nil {
		set("font_id", input.FontId)
	}
	if input.BorderColor != nil {
		set("border_color", *input.BorderColor)
	}
	if input.BorderWidth != nil {
		set("border_width", *input.BorderWidth)
	}
	if input.BorderRadius != nil {
		set("border_radius", *input.BorderRadius)
	}
	if input.BorderStyle != nil {
		set("border_style", *input.BorderStyle)
	}
	if input.Padding != nil {
		set("padding", *input.Padding)
	}
	if input.Margin != nil {
		set("margin", *input.Margin)
	}
	if input.TextShadow != nil {
		set("text_shadow", *input.TextShadow)
	}
	return updates
}

// styleDefaultsToUpdates builds the reset column→value map for the given target
// from the supplied defaults (the font FK always resets to nil).
func styleDefaultsToUpdates(target string, d VisualStyle) map[string]any {
	return map[string]any{
		target + "_bg_color":      d.BgColor,
		target + "_bg_opacity":    d.BgOpacity,
		target + "_text_color":    d.TextColor,
		target + "_font_id":       nil,
		target + "_border_color":  d.BorderColor,
		target + "_border_width":  d.BorderWidth,
		target + "_border_radius": d.BorderRadius,
		target + "_border_style":  d.BorderStyle,
		target + "_padding":       d.Padding,
		target + "_margin":        d.Margin,
		target + "_text_shadow":   d.TextShadow,
	}
}

// updateStyle writes the provided fields into the active theme's verse/couplet
// columns (target selects which), reloads, then broadcasts and returns the
// resulting style. UpdateVerseStyle/UpdateCoupletStyle are thin wrappers.
func (g *DbHandler) updateStyle(target string, input StyleInput) VisualStyle {
	t, err := loadActiveTheme()
	if err != nil {
		log.Printf("updateStyle(%s): error loading active theme: %v", target, err)
		return styleFromTheme(t, target)
	}
	if updates := styleInputToUpdates(target, input); len(updates) > 0 {
		if err := inits.DB.Model(&t).Updates(updates).Error; err != nil {
			log.Printf("updateStyle(%s): error saving: %v", target, err)
		}
		if reloaded, err := loadActiveTheme(); err == nil {
			t = reloaded
		}
	}
	style := styleFromTheme(t, target)
	g.styleB <- &StyleEvent{Type: "style_update", Target: target, Style: styleToMap(style), ThemeId: t.ID}
	return style
}

// resetStyle restores the active theme's verse/couplet columns (target selects
// which) to the given defaults, then broadcasts and returns them.
func (g *DbHandler) resetStyle(target string, d VisualStyle) VisualStyle {
	t, err := loadActiveTheme()
	if err != nil {
		log.Printf("resetStyle(%s): error loading active theme: %v", target, err)
		return d
	}
	if err := inits.DB.Model(&t).Updates(styleDefaultsToUpdates(target, d)).Error; err != nil {
		log.Printf("resetStyle(%s): error saving: %v", target, err)
	}
	g.styleB <- &StyleEvent{Type: "style_update", Target: target, Style: styleToMap(d), ThemeId: t.ID}
	return d
}

func (g *DbHandler) UpdateVerseStyle(input StyleInput) VisualStyle {
	return g.updateStyle("verse", input)
}

func (g *DbHandler) UpdateCoupletStyle(input StyleInput) VisualStyle {
	return g.updateStyle("couplet", input)
}

func (g *DbHandler) ResetVerseStyle() VisualStyle {
	return g.resetStyle("verse", DefaultVerseStyle)
}

func (g *DbHandler) ResetCoupletStyle() VisualStyle {
	return g.resetStyle("couplet", DefaultCoupletStyle)
}

// --- Theme CRUD ---

func (g *DbHandler) ListThemes() []models.Theme {
	var themes []models.Theme
	if err := inits.DB.Order("is_default DESC, id ASC").Find(&themes).Error; err != nil {
		log.Println("ListThemes: error", err)
		return nil
	}
	return themes
}

// GetActiveThemeId returns the id of the currently active theme (0 on error).
func (g *DbHandler) GetActiveThemeId() uint {
	gs := models.GlobalState{}
	if err := inits.DB.First(&gs, 1).Error; err != nil || gs.ActiveThemeId == nil {
		return 0
	}
	return *gs.ActiveThemeId
}

// applyTheme sets the active theme and broadcasts its styles. Shared by
// ApplyTheme and the create/duplicate paths (a new copy becomes active so the
// operator edits it right away).
func (g *DbHandler) applyTheme(t models.Theme) {
	gs := models.GlobalState{}
	if err := inits.DB.First(&gs, 1).Error; err != nil {
		log.Println("applyTheme: error reading GlobalState", err)
		return
	}
	if err := inits.DB.Model(&gs).Update("active_theme_id", t.ID).Error; err != nil {
		log.Println("applyTheme: error saving active theme", err)
		return
	}
	g.broadcastTheme(t)
}

func (g *DbHandler) ApplyTheme(idF float32) []models.Theme {
	var t models.Theme
	if err := inits.DB.First(&t, uint(idF)).Error; err != nil {
		log.Println("ApplyTheme: theme not found", err)
		return g.ListThemes()
	}
	g.applyTheme(t)
	return g.ListThemes()
}

// copyThemeStyle clones the style fields of src into a fresh, non-default theme
// with the given name, persists it, makes it active and returns it.
func (g *DbHandler) copyThemeStyle(src models.Theme, name string) *models.Theme {
	nt := src
	nt.Model = gorm.Model{}
	nt.Name = name
	nt.IsDefault = false
	if err := inits.DB.Create(&nt).Error; err != nil {
		log.Println("copyThemeStyle: error creating theme", err)
		return nil
	}
	g.applyTheme(nt)
	return &nt
}

// CreateTheme adds a new theme as a copy of the currently active one.
func (g *DbHandler) CreateTheme(name string) *models.Theme {
	src, err := loadActiveTheme()
	if err != nil {
		log.Println("CreateTheme: error loading active theme", err)
		return nil
	}
	return g.copyThemeStyle(src, name)
}

// DuplicateTheme adds a new theme as a copy of the given theme.
func (g *DbHandler) DuplicateTheme(idF float32) *models.Theme {
	var src models.Theme
	if err := inits.DB.First(&src, uint(idF)).Error; err != nil {
		log.Println("DuplicateTheme: theme not found", err)
		return nil
	}
	return g.copyThemeStyle(src, src.Name+" (копия)")
}

func (g *DbHandler) RenameTheme(idF float32, name string) []models.Theme {
	if err := inits.DB.Model(&models.Theme{}).Where("id = ?", uint(idF)).Update("name", name).Error; err != nil {
		log.Println("RenameTheme: error", err)
	}
	return g.ListThemes()
}

// DeleteTheme removes a theme. The default theme cannot be deleted. Deleting the
// active theme falls back to the default theme (re-broadcasting its styles).
func (g *DbHandler) DeleteTheme(idF float32) []models.Theme {
	id := uint(idF)
	var t models.Theme
	if err := inits.DB.First(&t, id).Error; err != nil {
		log.Println("DeleteTheme: theme not found", err)
		return g.ListThemes()
	}
	if t.IsDefault {
		log.Println("DeleteTheme: refusing to delete default theme")
		return g.ListThemes()
	}
	wasActive := g.GetActiveThemeId() == id

	// Outputs pointing at the theme being deleted fall back to the default
	// theme once its FK is nulled below — remember them so their (now
	// resolved) style can be re-broadcast afterwards.
	var affectedOutputs []models.Output
	inits.DB.Where("theme_id = ?", id).Find(&affectedOutputs)

	if err := inits.DB.Delete(&models.Theme{}, id).Error; err != nil {
		log.Println("DeleteTheme: error deleting", err)
		return g.ListThemes()
	}
	if err := inits.DB.Model(&models.Output{}).Where("theme_id = ?", id).Update("theme_id", nil).Error; err != nil {
		log.Println("DeleteTheme: error clearing output theme refs", err)
	}

	if wasActive {
		var def models.Theme
		if err := inits.DB.Where("is_default = ?", true).First(&def).Error; err == nil {
			g.applyTheme(def)
		}
	}
	if len(affectedOutputs) > 0 {
		// They all resolved to the same theme before deletion, so they all
		// fall back the same way — resolving once is enough. watchChannels
		// recomputes outputIds for every output currently on that theme, so
		// this stays correct (and idempotent) for outputs unaffected by the
		// deletion too.
		fallback := resolveOutputTheme(affectedOutputs[0])
		g.broadcastTheme(fallback)
	}
	return g.ListThemes()
}

func (g *DbHandler) UploadFont(name, mimeType string, data []byte) *models.Font {
	if len(data) > 5*1024*1024 {
		log.Println("UploadFont: file too large", len(data))
		return nil
	}
	validMimes := map[string]bool{
		"font/woff2":               true,
		"font/ttf":                 true,
		"application/font-woff2":   true,
		"application/x-font-ttf":   true,
		"application/octet-stream": true,
	}
	if !validMimes[mimeType] {
		log.Println("UploadFont: invalid mime type", mimeType)
		return nil
	}
	font := models.Font{
		Name:      name,
		MimeType:  mimeType,
		Data:      data,
		SizeBytes: len(data),
	}
	if err := inits.DB.Create(&font).Error; err != nil {
		log.Println("UploadFont: error creating font", err)
		return nil
	}
	g.broadcastFonts()
	return &font
}

// broadcastFonts pushes the current font list (metadata only — Data blobs are
// omitted and served via /font/{id}) to the receiver.
func (g *DbHandler) broadcastFonts() {
	var fonts []models.Font
	inits.DB.Omit("Data").Find(&fonts)
	g.styleB <- &StyleEvent{Type: "fonts_changed", Fonts: fonts}
}

func (g *DbHandler) DeleteFont(idF float32) {
	id := uint(idF)
	if err := inits.DB.Delete(&models.Font{}, id).Error; err != nil {
		log.Println("DeleteFont: error deleting font", err)
		return
	}
	// Null out any theme style FK pointing to this font, across all themes.
	inits.DB.Model(&models.Theme{}).Where("verse_font_id = ?", id).Update("verse_font_id", nil)
	inits.DB.Model(&models.Theme{}).Where("couplet_font_id = ?", id).Update("couplet_font_id", nil)
	// Re-broadcast the active theme so a live projection using the deleted font
	// falls back immediately.
	if t, err := loadActiveTheme(); err == nil {
		g.broadcastTheme(t)
	}

	g.broadcastFonts()
}

func (g *DbHandler) getFontDataInternal(id uint) ([]byte, string, error) {
	var f models.Font
	if err := inits.DB.First(&f, id).Error; err != nil {
		return nil, "", err
	}
	return f.Data, f.MimeType, nil
}

// --- Backdrop images ---

func (g *DbHandler) UploadImage(name, mimeType string, data []byte) *models.Image {
	if len(data) > 10*1024*1024 {
		log.Println("UploadImage: file too large", len(data))
		return nil
	}
	validMimes := map[string]bool{
		"image/png":  true,
		"image/jpeg": true,
		"image/webp": true,
		"image/gif":  true,
	}
	if !validMimes[mimeType] {
		log.Println("UploadImage: invalid mime type", mimeType)
		return nil
	}
	image := models.Image{
		Name:      name,
		MimeType:  mimeType,
		Data:      data,
		SizeBytes: len(data),
	}
	if err := inits.DB.Create(&image).Error; err != nil {
		log.Println("UploadImage: error creating image", err)
		return nil
	}
	return &image
}

func (g *DbHandler) DeleteImage(idF float32) {
	id := uint(idF)
	if err := inits.DB.Delete(&models.Image{}, id).Error; err != nil {
		log.Println("DeleteImage: error deleting image", err)
		return
	}
	// Null out any theme backdrop FK pointing to this image, across all themes.
	inits.DB.Model(&models.Theme{}).Where("bg_image_id = ?", id).Update("bg_image_id", nil)
	// Re-broadcast the active theme so a live projection using the deleted image
	// falls back immediately.
	if t, err := loadActiveTheme(); err == nil {
		g.broadcastTheme(t)
	}
}

func (g *DbHandler) getImageDataInternal(id uint) ([]byte, string, error) {
	var im models.Image
	if err := inits.DB.First(&im, id).Error; err != nil {
		return nil, "", err
	}
	return im.Data, im.MimeType, nil
}
