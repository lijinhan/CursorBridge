package bridge

import (
	"path/filepath"
	"runtime"

	"cursorbridge/internal/certs"
)

func detectCAInstallMode() string {
	switch runtime.GOOS {
	case "windows":
		return "auto-user-store"
	case "darwin":
		return "auto-login-keychain"
	case "linux":
		if certs.IsInstalledUserRoot() {
			return "auto-nss"
		}
		return "manual-or-nss"
	default:
		return "manual"
	}
}

func buildCAWarning() string {
	switch runtime.GOOS {
	case "linux":
		if certs.IsInstalledUserRoot() {
			return "Linux 信任目前通过用户的 NSS 数据库 (~/.pki/nssdb) 配置。其他应用可能仍需手动将 CA PEM 从设置文件夹导入系统信任存储。"
		}
		return "Linux has no unified cross-desktop trust store. CursorBridge can import into the user's NSS DB after installing certutil, but other apps may still require manual CA PEM import from the settings folder."
	case "darwin":
		return "macOS 信任已安装到当前用户的登录钥匙串。如果钥匙串请求权限，请批准以使 Cursor 信任本地 MITM CA。"
	default:
		return ""
	}
}

func (s *ProxyService) InstallCA() (ProxyState, error) {
	if err := certs.InstallUserRoot(filepath.Join(s.ca.Dir(), "ca.crt")); err != nil {
		s.mu.Lock()
		s.state.LastError = err.Error()
		s.mu.Unlock()
		return s.GetState(), err
	}
	s.mu.Lock()
	s.state.CAInstalled = true
	s.state.LastError = ""
	s.mu.Unlock()
	return s.GetState(), nil
}

func (s *ProxyService) UninstallCA() (ProxyState, error) {
	if err := certs.UninstallUserRoot(); err != nil {
		s.mu.Lock()
		s.state.LastError = err.Error()
		s.mu.Unlock()
		return s.GetState(), err
	}
	s.mu.Lock()
	s.state.CAInstalled = false
	s.state.LastError = ""
	s.mu.Unlock()
	return s.GetState(), nil
}

func (s *ProxyService) ExportCAPEM() string {
	return string(s.ca.CertPEM())
}