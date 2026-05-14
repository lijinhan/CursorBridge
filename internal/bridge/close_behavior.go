package bridge

import (
	"encoding/json"
	"path/filepath"

	"cursorbridge/internal/safefile"
)

func (s *ProxyService) SetStateCallback(cb func(running bool)) {
	s.mu.Lock()
	s.onStateChange = cb
	s.mu.Unlock()
}

func (s *ProxyService) SetQuitCallback(cb func()) {
	s.mu.Lock()
	s.quitCb = cb
	s.mu.Unlock()
}

func (s *ProxyService) SetHideCallback(cb func()) {
	s.mu.Lock()
	s.hideCb = cb
	s.mu.Unlock()
}

func (s *ProxyService) GetCloseAction() string {
	cfg, err := readConfig(s.cfgDir)
	if err != nil {
		return ""
	}
	switch cfg.CloseAction {
	case "quit", "tray":
		return cfg.CloseAction
	default:
		return ""
	}
}

func (s *ProxyService) SetCloseAction(action string) error {
	cfg, err := readConfig(s.cfgDir)
	if err != nil {
		return err
	}
	switch action {
	case "quit", "tray":
		cfg.CloseAction = action
	default:
		cfg.CloseAction = ""
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return safefile.Write(filepath.Join(s.cfgDir, "config.json"), b, 0o600)
}

func (s *ProxyService) RequestQuit() {
	s.mu.RLock()
	cb := s.quitCb
	s.mu.RUnlock()
	if cb != nil {
		go cb()
	}
}

func (s *ProxyService) RequestHide() {
	s.mu.RLock()
	cb := s.hideCb
	s.mu.RUnlock()
	if cb != nil {
		go cb()
	}
}

func (s *ProxyService) fireState() {
	cb := s.onStateChange
	running := s.state.Running
	if cb != nil {
		cb(running)
	}
}