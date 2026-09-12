package main

import (
	"fmt"

	"changeme/backend/inits"
	"changeme/backend/models"
)

// AudioDevice — один пункт списка устройств вывода вкладки «Звук» (этап 3).
// ID — malgo DeviceID.String(); "" зарезервирована за системным по
// умолчанию и ListDevices всегда ставит её первой записью.
type AudioDevice struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
	// Missing — устройство было выбрано (GlobalState.AudioDeviceId), но не
	// нашлось в свежем перечислении: выдернули, отключили в системе и т.п.
	// Сопоставление только по ID.String() — см. defaultOpenDevice.
	Missing bool `json:"missing"`
}

// ListDevices перечисляет устройства вывода. Системное по умолчанию — всегда
// первым пунктом, независимо от того, что вернул malgo. Если сейчас выбрано
// (SetDevice/GlobalState) устройство, которого нет в свежем списке — в конец
// добавляется запись с Missing: true, чтобы UI мог показать «выбранное
// устройство пропало», а не молча забыть о выборе оператора.
func (a *AudioService) ListDevices() []AudioDevice {
	selected := a.GetDeviceId()
	devices := []AudioDevice{{ID: "", Name: systemDefaultDeviceName, IsDefault: true}}

	infos, err := a.pl.listPlaybackDevices()
	if err != nil {
		// Отсутствие звуковой карты/бэкенда — не повод падать: список из
		// одного системного пункта достаточен, чтобы вкладка «Звук» вообще
		// открылась (план: старт на машине без звуковой карты не должен
		// ломаться).
		infos = nil
	}

	foundSelected := selected == ""
	for _, info := range infos {
		id := info.ID.String()
		if id == selected {
			foundSelected = true
		}
		devices = append(devices, AudioDevice{ID: id, Name: info.Name(), IsDefault: info.IsDefault != 0})
	}
	if !foundSelected {
		devices = append(devices, AudioDevice{ID: selected, Missing: true})
	}
	return devices
}

// GetDeviceId возвращает текущий выбор (malgo DeviceID.String(), "" —
// системное по умолчанию).
func (a *AudioService) GetDeviceId() string {
	return a.pl.selectedDevice()
}

// SetDevice меняет устройство вывода. Смена ВСЕГДА останавливает
// воспроизведение (план, этап 3, осознанное решение) — продолжение с той же
// позиции на другом устройстве не входит в первую версию, и попытка
// «бесшовно» перевести играющий трек добавила бы риска ради сценария,
// которого никто не просил. Устройство закрывается (closeDevice, вне p.mu —
// И2), чтобы следующий Play() поднял именно новое через ensureDevice, а не
// продолжил молча играть на старом.
func (a *AudioService) SetDevice(id string) error {
	a.invalidatePending()
	if oldSrc := a.pl.stop(); oldSrc != nil {
		oldSrc.Close()
	}
	a.pl.closeDevice()
	a.pl.setSelectedDevice(id)

	if err := inits.DB.Model(&models.GlobalState{}).Where("id = ?", 1).Update("audio_device_id", id).Error; err != nil {
		return fmt.Errorf("не удалось сохранить выбор устройства: %w", err)
	}
	a.emit("audio_stopped", nil)
	return nil
}
