package ipcheck

import (
	"context"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"
)

type stageName int

const (
	stageUnknown stageName = iota
	stageValid
	stageRTT
	stageSpeed
	stageExit
)

type signalController struct {
	mu              sync.Mutex
	currentStage    stageName
	cancelStage     context.CancelFunc
	lastInterruptAt time.Time
	cachedIPs       []IPInfo
}

func newSignalController() *signalController {
	return &signalController{}
}

func (s *signalController) setStage(stage stageName, cancel context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentStage = stage
	s.cancelStage = cancel
	s.cachedIPs = nil
}

func (s *signalController) clearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cachedIPs = nil
}

func (s *signalController) clearStage() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentStage = stageUnknown
	s.cancelStage = nil
}

func (s *signalController) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentStage = stageExit
	s.cancelStage = nil
}

func (s *signalController) cache(info IPInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cachedIPs = append(s.cachedIPs, info)
}

func (s *signalController) printCache() {
	s.mu.Lock()
	cached := make([]IPInfo, len(s.cachedIPs))
	copy(cached, s.cachedIPs)
	s.mu.Unlock()

	if len(cached) == 0 {
		return
	}
	sort.Slice(cached, func(i, j int) bool {
		if cached[i].MaxSpeed != cached[j].MaxSpeed {
			return cached[i].MaxSpeed > cached[j].MaxSpeed
		}
		if cached[i].RTT != cached[j].RTT {
			return cached[i].RTT < cached[j].RTT
		}
		return cached[i].CountryCity < cached[j].CountryCity
	})
	consolePrint("当前测试阶段IP 信息如下:")
	for _, info := range cached {
		consolePrint(info.infoString())
	}
}

func (s *signalController) handleInterrupt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if !s.lastInterruptAt.IsZero() && now.Sub(s.lastInterruptAt) < 1500*time.Millisecond {
		os.Exit(130)
	}
	s.lastInterruptAt = now
	if s.currentStage == stageUnknown || s.currentStage == stageExit {
		os.Exit(130)
	}
	if s.cancelStage != nil {
		s.cancelStage()
	}
}

func installSignalHandler(ctrl *signalController) func() {
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range ch {
			ctrl.handleInterrupt()
		}
	}()
	return func() {
		signal.Stop(ch)
		close(ch)
	}
}
