package ipcheck

import (
	"context"
	"os"
	"os/signal"
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
}

func newSignalController() *signalController {
	return &signalController{}
}

func (s *signalController) setStage(stage stageName, cancel context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentStage = stage
	s.cancelStage = cancel
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
