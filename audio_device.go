package main

import (
	"errors"
	"fmt"
	"log"
	"unsafe"

	"github.com/gen2brain/malgo"
	"github.com/gopxl/beep/v2"
)

// unsafeFloat32Slice реинтерпретирует байтовый буфер malgo как срез
// float32 нужной длины — единственное место во всём движке, где нужен
// unsafe, потому что malgo отдаёт колбэку []byte независимо от
// сконфигурированного формата сэмплов.
func unsafeFloat32Slice(b []byte, n int) []float32 {
	return unsafe.Slice((*float32)(unsafe.Pointer(&b[0])), n)
}

// ensureDevice лениво поднимает устройство вывода при первом обращении
// (Play). Открытие трогает системное аудио — вызов openDevice всегда
// выполняется вне p.mu (И2): контекст/устройство создаются и стартуют
// снаружи, под mu только фиксируется результат.
//
// Вся функция сериализована отдельным p.openMu (НЕ p.mu — аудио-колбэк его
// не берёт, И2 не страдает). Раньше без этой сериализации два быстрых
// клика "играть" на первом за сессию воспроизведении поднимали ДВА
// устройства параллельно (оба видели p.deviceOpen==false и оба звали
// openDevice), и "проигравший" вызывал dev.Stop() на своей копии — но
// Stop-колбэк malgo привязан к p (не к конкретному dev), так что событие
// "устройство пропало" улетало в lost и глушило только что начавшийся
// трек победителя ложной аварией. Теперь второй вызов просто ждёт первый
// на openMu и видит уже готовый p.deviceOpen — открывать нечего.
func (p *player) ensureDevice() error {
	p.openMu.Lock()
	defer p.openMu.Unlock()

	p.mu.Lock()
	open := p.deviceOpen
	p.mu.Unlock()
	if open {
		return nil
	}

	ctx, dev, rate, err := p.openDevice()
	if err != nil {
		return err
	}

	p.mu.Lock()
	p.ctx, p.dev = ctx, dev
	p.devRate = beep.SampleRate(rate)
	p.deviceOpen = true
	// Сбрасываем на каждое успешное открытие: closeDevice взводит его в
	// true при плановом останове (см. ниже), а иначе после первого же
	// closeDevice/ensureDevice детект пропажи устройства (этап 3) был бы
	// мёртв навсегда — expectStop общий на весь плеер, не на одно
	// устройство.
	p.expectStop.Store(false)
	p.mu.Unlock()
	return nil
}

// defaultOpenDevice — реальная реализация p.openDevice. Вынесена отдельной
// функцией, а не встроена в ensureDevice, чтобы тесты (TestAudioServiceWithoutDevice)
// могли подменить её на функцию, возвращающую ошибку, без единого вызова
// malgo — И6 в применении к самому устройству, не только к DSP-цепочке.
func (p *player) defaultOpenDevice() (*malgo.AllocatedContext, *malgo.Device, uint32, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(msg string) {
		log.Println("malgo:", msg) // дублирующий канал для dev-сборки, см. build/AGENTS.md про -H windowsgui
	})
	if err != nil {
		return nil, nil, 0, fmt.Errorf("не удалось инициализировать аудио-контекст: %w", err)
	}

	cfg := malgo.DefaultDeviceConfig(malgo.Playback)
	cfg.Playback.Format = malgo.FormatF32
	cfg.Playback.Channels = 2
	// Частота устройства — родная (miniaudio сам возьмёт частоту эндпоинта),
	// а не жёсткие 48000: в shared-режиме WASAPI встроенные карты часто
	// работают на 44100, и жёсткая частота гнала бы 44.1->48 в beep, а затем
	// miniaudio ещё раз 48->44.1 — двойной ресемплинг.
	cfg.SampleRate = 0

	dev, err := malgo.InitDevice(ctx.Context, cfg, malgo.DeviceCallbacks{
		Data: p.onSamples,
		Stop: p.onDeviceStopped,
	})
	if err != nil {
		ctx.Uninit()
		ctx.Free()
		if errors.Is(err, malgo.ErrAlreadyInUse) {
			return nil, nil, 0, fmt.Errorf("устройство вывода занято другим приложением: %w", err)
		}
		return nil, nil, 0, fmt.Errorf("не удалось открыть устройство вывода: %w", err)
	}

	if err := dev.Start(); err != nil {
		dev.Uninit()
		ctx.Uninit()
		ctx.Free()
		return nil, nil, 0, fmt.Errorf("не удалось запустить устройство вывода: %w", err)
	}

	return ctx, dev, dev.SampleRate(), nil
}

// closeDevice останавливает устройство и освобождает malgo-контекст.
// Вызывается из ServiceShutdown. Паттерн И2: под p.mu только вынимаем
// указатели и обнуляем поля, Stop()/Uninit() — уже вне mu, иначе
// Stop() (ждёт worker-поток) и наш же data-колбэк (выполняется на нём же)
// дедлочат друг друга навсегда. Идемпотентно: повторный вызов видит dev==nil
// и ничего не делает.
func (p *player) closeDevice() {
	p.mu.Lock()
	dev, ctx := p.dev, p.ctx
	p.dev, p.ctx, p.deviceOpen = nil, nil, false
	p.expectStop.Store(true) // это наш останов, не пропажа устройства (этап 3)
	p.mu.Unlock()

	if dev != nil {
		dev.Stop()
		dev.Uninit()
	}
	if ctx != nil {
		ctx.Uninit()
		ctx.Free()
	}
}
