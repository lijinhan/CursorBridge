package certs

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const caCommonName = "CursorBridge Local CA"

// InstallUserRoot adds the CA at certPath into the current-user trusted root
// store. Windows uses the user Root store, macOS uses the login keychain, and
// Linux uses the user's NSS DB when certutil is available.
func InstallUserRoot(certPath string) error {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("certutil", "-user", "-addstore", "Root", certPath)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("certutil 添加证书: %w (%s)", err, trim(out))
		}
		return nil
	case "darwin":
		keychain, err := darwinLoginKeychain()
		if err != nil {
			return err
		}
		cmd := exec.Command("security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", keychain, certPath)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("security 添加信任证书: %w (%s)", err, trim(out))
		}
		return nil
	case "linux":
		db, err := linuxEnsureNSSDB()
		if err != nil {
			return err
		}
		_ = exec.Command("certutil", "-D", "-d", db, "-n", caCommonName).Run()
		cmd := exec.Command("certutil", "-A", "-d", db, "-n", caCommonName, "-t", "C,,", "-i", certPath)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("certutil NSS 添加: %w (%s)", err, trim(out))
		}
		return nil
	default:
		return errors.New("此平台不支持 CA 自动安装")
	}
}

// UninstallUserRoot removes our CA from the current-user trusted root store.
func UninstallUserRoot() error {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("certutil", "-user", "-delstore", "Root", caCommonName)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("certutil 添加证书: %w (%s)", err, trim(out))
		}
		return nil
	case "darwin":
		keychain, err := darwinLoginKeychain()
		if err != nil {
			return err
		}
		certPath, err := darwinInstalledCertTempFile()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		defer os.Remove(certPath)
		cmd := exec.Command("security", "remove-trusted-cert", "-d", certPath, keychain)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("security 删除信任证书: %w (%s)", err, trim(out))
		}
		return nil
	case "linux":
		db, err := linuxNSSDBPath()
		if err != nil {
			return err
		}
		if _, err := exec.LookPath("certutil"); err != nil {
			return nil
		}
		cmd := exec.Command("certutil", "-D", "-d", db, "-n", caCommonName)
		out, err := cmd.CombinedOutput()
		if err != nil {
			msg := string(out)
			if strings.Contains(strings.ToLower(msg), "could not find cert") || strings.Contains(strings.ToLower(msg), "not found") {
				return nil
			}
			return fmt.Errorf("certutil NSS 删除: %w (%s)", err, trim(out))
		}
		return nil
	default:
		return errors.New("此平台不支持 CA 卸载")
	}
}

// IsInstalledUserRoot reports whether our CA is present in the current-user
// trusted root store. Matching is done by CA common name, which is stable
// across CA rebuilds because we never change it.
func IsInstalledUserRoot() bool {
	switch runtime.GOOS {
	case "windows":
		out, err := exec.Command("certutil", "-user", "-store", "Root").CombinedOutput()
		if err != nil {
			return false
		}
		return strings.Contains(string(out), caCommonName)
	case "darwin":
		keychain, err := darwinLoginKeychain()
		if err != nil {
			return false
		}
		out, err := exec.Command("security", "find-certificate", "-a", "-c", caCommonName, "-Z", keychain).CombinedOutput()
		if err != nil {
			return false
		}
		return strings.Contains(string(out), caCommonName)
	case "linux":
		db, err := linuxNSSDBPath()
		if err != nil {
			return false
		}
		if _, err := exec.LookPath("certutil"); err != nil {
			return false
		}
		out, err := exec.Command("certutil", "-L", "-d", db, "-n", caCommonName).CombinedOutput()
		if err != nil {
			return false
		}
		return strings.Contains(string(out), caCommonName)
	default:
		return false
	}
}

func darwinLoginKeychain() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("解析主目录: %w", err)
	}
	return filepath.Join(home, "Library", "Keychains", "login.keychain-db"), nil
}

func darwinInstalledCertTempFile() (string, error) {
	keychain, err := darwinLoginKeychain()
	if err != nil {
		return "", err
	}
	out, err := exec.Command("security", "find-certificate", "-a", "-c", caCommonName, "-p", keychain).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("security 查找证书: %w (%s)", err, trim(out))
	}
	pem := strings.TrimSpace(string(out))
	if pem == "" {
		return "", os.ErrNotExist
	}
	f, err := os.CreateTemp("", "cursorbridge-ca-*.crt")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(pem + "\n"); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func linuxNSSDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("解析主目录: %w", err)
	}
	return "sql:" + filepath.Join(home, ".pki", "nssdb"), nil
}

func linuxEnsureNSSDB() (string, error) {
	if _, err := exec.LookPath("certutil"); err != nil {
		return "", errors.New("Linux CA 导入需要 certutil（通常来自 libnss3-tools / nss-tools）；请安装它，或按照 UI 中显示的手动系统存储步骤操作")
	}
	db, err := linuxNSSDBPath()
	if err != nil {
		return "", err
	}
	dir := strings.TrimPrefix(db, "sql:")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(dir, "cert9.db")); os.IsNotExist(err) {
		cmd := exec.Command("certutil", "-N", "-d", db, "--empty-password")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("certutil 初始化 NSS 数据库: %w (%s)", err, trim(out))
		}
	}
	return db, nil
}

func trim(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
