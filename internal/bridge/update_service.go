package bridge

import (
	"context"
	"sync"

	"cursorbridge/internal/updater"
)

const appVersionTag = "v1.0.1"

type UpdateState struct {
	Checking   bool              `json:"checking"`
	Available  bool              `json:"available"`
	Downloading bool             `json:"downloading"`
	Installing bool              `json:"installing"`
	Info       *updater.UpdateInfo `json:"info,omitempty"`
	Progress   *updater.DownloadProgress `json:"progress,omitempty"`
	Error      string            `json:"error,omitempty"`
}

type UpdateService struct {
	mu     sync.Mutex
	state  UpdateState
	quitFn func()
}

func NewUpdateService(quitFn func()) *UpdateService {
	return &UpdateService{quitFn: quitFn}
}

func (u *UpdateService) GetState() UpdateState {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.state
}

func (u *UpdateService) CheckForUpdates() UpdateState {
	u.mu.Lock()
	if u.state.Checking || u.state.Downloading || u.state.Installing {
		u.mu.Unlock()
		return u.state
	}
	u.state = UpdateState{Checking: true}
	u.mu.Unlock()

	info, err := updater.Check(context.Background(), appVersionTag)

	u.mu.Lock()
	defer u.mu.Unlock()
	if err != nil {
		u.state = UpdateState{Error: err.Error()}
		return u.state
	}
	u.state = UpdateState{
		Available: info.HasUpdate,
		Info:      &info,
	}
	return u.state
}

func (u *UpdateService) DownloadUpdate() UpdateState {
	u.mu.Lock()
	info := u.state.Info
	if info == nil || !info.HasUpdate || u.state.Downloading || u.state.Installing {
		u.mu.Unlock()
		return u.state
	}
	u.state.Downloading = true
	u.state.Progress = nil
	u.mu.Unlock()

	path, err := updater.Download(context.Background(), *info, func(p updater.DownloadProgress) {
		u.mu.Lock()
		u.state.Progress = &p
		u.mu.Unlock()
	})

	u.mu.Lock()
	defer u.mu.Unlock()
	if err != nil {
		u.state = UpdateState{Error: err.Error(), Info: info}
		return u.state
	}
	u.state.Downloading = false
	u.state.Available = true
	u.state.Info = info
	u.state.Info.Filename = path // store local path for install step
	u.state.Progress = &updater.DownloadProgress{Percent: 100, Path: path}
	return u.state
}

func (u *UpdateService) InstallUpdate() UpdateState {
	u.mu.Lock()
	info := u.state.Info
	if info == nil || u.state.Installing {
		u.mu.Unlock()
		return u.state
	}
	u.state.Installing = true
	u.mu.Unlock()

	newBinary, err := updater.PrepareInstall(info.Filename)
	if err != nil {
		u.mu.Lock()
		u.state = UpdateState{Error: err.Error(), Info: info}
		u.mu.Unlock()
		return u.state
	}

	err = updater.InstallAndRestart(newBinary, u.quitFn)
	u.mu.Lock()
	defer u.mu.Unlock()
	if err != nil {
		u.state = UpdateState{Error: err.Error(), Info: info}
		return u.state
	}
	// If we reach here, the install script was launched and quitFn was called.
	// The app will exit shortly.
	u.state.Installing = true
	return u.state
}

func (u *UpdateService) DismissUpdate() UpdateState {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.state = UpdateState{}
	return u.state
}