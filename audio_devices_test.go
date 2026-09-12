package main

import (
	"context"
	"path/filepath"
	"testing"

	"changeme/backend/inits"
	"changeme/backend/models"

	"github.com/gen2brain/malgo"
)

// TestListDevicesMarksMissingSelection — план: не нашли выбранное устройство
// в свежем перечислении -> запись с Missing: true, само сопоставление ТОЛЬКО
// по ID.String() (см. defaultOpenDevice/findDeviceById).
func TestListDevicesMarksMissingSelection(t *testing.T) {
	setupPlayerTestDB(t)
	a := NewAudioService()

	var id1, id2, id3 malgo.DeviceID
	id1[0], id2[0], id3[0] = 0x01, 0x02, 0x03

	a.pl.listPlaybackDevices = func() ([]malgo.DeviceInfo, error) {
		return []malgo.DeviceInfo{{ID: id1, IsDefault: 1}, {ID: id2}}, nil
	}

	devices := a.ListDevices()
	if devices[0].ID != "" || !devices[0].IsDefault {
		t.Fatalf("first entry must be the empty-id system default: %+v", devices[0])
	}
	for _, d := range devices {
		if d.Missing {
			t.Errorf("nothing should be missing before any selection: %+v", d)
		}
	}

	if err := a.SetDevice(id3.String()); err != nil {
		t.Fatalf("SetDevice: %v", err)
	}
	devices = a.ListDevices()
	found := false
	for _, d := range devices {
		if d.ID == id3.String() {
			found = true
			if !d.Missing {
				t.Errorf("device absent from enumeration must be Missing: %+v", d)
			}
		}
	}
	if !found {
		t.Fatal("selected-but-missing device not present in ListDevices output")
	}

	if err := a.SetDevice(id2.String()); err != nil {
		t.Fatalf("SetDevice: %v", err)
	}
	devices = a.ListDevices()
	for _, d := range devices {
		if d.ID == id2.String() && d.Missing {
			t.Errorf("device present in enumeration must not be Missing: %+v", d)
		}
	}
}

// TestSetDevicePersistsSelection — GetDeviceId/GlobalState.AudioDeviceId
// переживают выбор (план: "перезапустить — выбор пережил").
func TestSetDevicePersistsSelection(t *testing.T) {
	setupPlayerTestDB(t)
	a := NewAudioService()

	if got := a.GetDeviceId(); got != "" {
		t.Fatalf("initial device id = %q, want empty (system default)", got)
	}
	if err := a.SetDevice("deadbeef"); err != nil {
		t.Fatalf("SetDevice: %v", err)
	}
	if got := a.GetDeviceId(); got != "deadbeef" {
		t.Errorf("GetDeviceId = %q, want deadbeef", got)
	}

	var gs models.GlobalState
	if err := inits.DB.First(&gs, 1).Error; err != nil {
		t.Fatalf("read GlobalState: %v", err)
	}
	if gs.AudioDeviceId != "deadbeef" {
		t.Errorf("GlobalState.AudioDeviceId = %q, want deadbeef", gs.AudioDeviceId)
	}
}

// TestSetDeviceStopsPlayback — план (осознанное решение): смена устройства
// всегда останавливает воспроизведение.
func TestSetDeviceStopsPlayback(t *testing.T) {
	setupPlayerTestDB(t)
	a := NewAudioService()
	a.pl.openDevice = fakeOpenDevice(8000)

	src := filepath.Join(t.TempDir(), "t.wav")
	writeTestWav(t, src, uniqueTestDuration())
	res := a.ImportTrack(context.Background(), src)
	if res.Error != "" {
		t.Fatalf("ImportTrack: %s", res.Error)
	}
	playlist := a.CreatePlaylist("Тест")
	p := a.AddToPlaylist(float32(playlist.ID), float32(res.Track.ID))

	if err := a.Play(float32(playlist.ID), float32(p.Items[0].ID)); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if st := a.State(); st.Status != string(statusPlaying) {
		t.Fatalf("expected playing, got %q", st.Status)
	}

	if err := a.SetDevice("someotherdevice"); err != nil {
		t.Fatalf("SetDevice: %v", err)
	}
	if st := a.State(); st.Status != string(statusIdle) {
		t.Errorf("SetDevice must stop playback, status = %q", st.Status)
	}
}

// TestDeviceLostMarksStateAndStops — план: пропажа устройства (StopProc при
// expectStop==false) переводит в idle и выставляет DeviceLost, без тихой
// подмены на другое устройство.
func TestDeviceLostMarksStateAndStops(t *testing.T) {
	setupPlayerTestDB(t)
	a := NewAudioService()
	a.pl.openDevice = fakeOpenDevice(8000)

	src := filepath.Join(t.TempDir(), "t.wav")
	writeTestWav(t, src, uniqueTestDuration())
	res := a.ImportTrack(context.Background(), src)
	if res.Error != "" {
		t.Fatalf("ImportTrack: %s", res.Error)
	}
	playlist := a.CreatePlaylist("Тест")
	p := a.AddToPlaylist(float32(playlist.ID), float32(res.Track.ID))

	if err := a.Play(float32(playlist.ID), float32(p.Items[0].ID)); err != nil {
		t.Fatalf("Play: %v", err)
	}

	// Симулирует реальную пропажу устройства: StopProc malgo с
	// expectStop==false (наш собственный Stop()/SetDevice() его сперва
	// выставляют в true — см. closeDevice).
	a.pl.onDeviceStopped()

	waitForState(t, a, func(st PlayerState) bool {
		return st.Status == string(statusIdle) && st.DeviceLost
	})
}
