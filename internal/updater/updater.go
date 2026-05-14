// Package updater checks GitHub Releases for new versions, downloads the
// platform-appropriate artifact, verifies its SHA-256 checksum, and prepares
// it for in-place replacement of the running binary.
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"cursorbridge/internal/appdir"
	"cursorbridge/internal/debuglog"
)

const (
	githubRepo   = "lijinhan/CursorBridge"
	checkTimeout = 15 * time.Second
)

// UpdateInfo describes an available update returned by Check.
type UpdateInfo struct {
	HasUpdate    bool   `json:"hasUpdate"`
	CurrentTag   string `json:"currentTag"`
	LatestTag    string `json:"latestTag"`
	DownloadURL  string `json:"downloadURL,omitempty"`
	Filename     string `json:"filename,omitempty"`
	FileSize     int64  `json:"fileSize,omitempty"`
	SHA256URL    string `json:"sha256URL,omitempty"`
	ChangelogURL string `json:"changelogURL,omitempty"`
}

// DownloadProgress is emitted periodically during a download so the UI can
// render a progress bar.
type DownloadProgress struct {
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Percent    int    `json:"percent"`
	Path       string `json:"path"`
}

// githubRelease is the subset of the GitHub API response we care about.
type githubRelease struct {
	TagName string         `json:"tag_name"`
	HTMLURL string         `json:"html_url"`
	Assets  []githubAsset  `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// platformSuffix returns the asset filename suffix for the current OS/arch.
// This must match the naming convention in .github/workflows/release.yml.
func platformSuffix() string {
	osName := runtime.GOOS
	arch := runtime.GOOS
	switch runtime.GOOS {
	case "windows":
		osName = "windows"
		arch = "amd64"
	case "darwin":
		osName = "macos-universal"
		arch = ""
	case "linux":
		osName = "linux"
		arch = "amd64"
	default:
		osName = runtime.GOOS
		arch = runtime.GOARCH
	}
	if arch != "" {
		return fmt.Sprintf("-%s-%s", osName, arch)
	}
	return "-" + osName
}

// assetName returns the expected asset filename for the current platform.
func assetName(version string) string {
	suffix := platformSuffix()
	switch runtime.GOOS {
	case "windows":
		return fmt.Sprintf("CursorBridge-%s%s.zip", version, suffix)
	case "darwin":
		return fmt.Sprintf("CursorBridge-%s%s.zip", version, suffix)
	default:
		return fmt.Sprintf("CursorBridge-%s%s.tar.gz", version, suffix)
	}
}

// sha256AssetName returns the expected SHA-256 checksum asset filename.
func sha256AssetName(version string) string {
	return assetName(version) + ".sha256"
}

// parseSemver extracts [major, minor, patch] from a tag like "v1.2.3".
func parseSemver(tag string) (int, int, int) {
	s := strings.TrimPrefix(tag, "v")
	s = strings.SplitN(s, "-", 2)[0]
	parts := strings.SplitN(s, ".", 3)
	maj, min, pat := 0, 0, 0
	if len(parts) >= 1 {
		fmt.Sscanf(parts[0], "%d", &maj)
	}
	if len(parts) >= 2 {
		fmt.Sscanf(parts[1], "%d", &min)
	}
	if len(parts) >= 3 {
		fmt.Sscanf(parts[2], "%d", &pat)
	}
	return maj, min, pat
}

// isNewer returns true if remote tag is strictly newer than local tag.
func isNewer(remote, local string) bool {
	rm, rn, rp := parseSemver(remote)
	lm, ln, lp := parseSemver(local)
	if rm != lm {
		return rm > lm
	}
	if rn != ln {
		return rn > ln
	}
	return rp > lp
}

// Check queries the GitHub Releases API and returns update info.
// Returns UpdateInfo with HasUpdate=false if already on the latest version.
func Check(ctx context.Context, currentTag string) (UpdateInfo, error) {
	info := UpdateInfo{
		CurrentTag: currentTag,
	}
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return info, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return info, fmt.Errorf("github api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return info, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return info, fmt.Errorf("github returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return info, fmt.Errorf("decode release: %w", err)
	}

	info.LatestTag = release.TagName
	info.ChangelogURL = release.HTMLURL

	if !isNewer(release.TagName, currentTag) {
		return info, nil
	}

	version := strings.TrimPrefix(release.TagName, "v")
	targetName := assetName(version)
	shaName := sha256AssetName(version)

	for _, a := range release.Assets {
		if a.Name == targetName {
			info.DownloadURL = a.BrowserDownloadURL
			info.Filename = a.Name
			info.FileSize = a.Size
		}
		if a.Name == shaName {
			info.SHA256URL = a.BrowserDownloadURL
		}
	}

	if info.DownloadURL == "" {
		return info, fmt.Errorf("no matching asset found for %s", targetName)
	}

	info.HasUpdate = true
	return info, nil
}

// Download fetches the update archive to the app's config directory and
// verifies the SHA-256 checksum. The onProgress callback is called
// periodically with download progress.
func Download(ctx context.Context, info UpdateInfo, onProgress func(DownloadProgress)) (string, error) {
	if info.DownloadURL == "" {
		return "", fmt.Errorf("no download URL")
	}

	cfgDir, err := appdir.ConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	updatesDir := filepath.Join(cfgDir, "updates")
	if err := os.MkdirAll(updatesDir, 0o755); err != nil {
		return "", fmt.Errorf("create updates dir: %w", err)
	}

	archivePath := filepath.Join(updatesDir, info.Filename)
	if err := downloadFile(ctx, info.DownloadURL, archivePath, info.FileSize, onProgress); err != nil {
		return "", fmt.Errorf("download: %w", err)
	}

	if err := verifyChecksum(ctx, archivePath, info.SHA256URL); err != nil {
		os.Remove(archivePath)
		return "", fmt.Errorf("checksum: %w", err)
	}

	debuglog.Printf("[UPDATER] download complete: %s", archivePath)
	return archivePath, nil
}

// downloadFile downloads url to destPath, calling onProgress periodically.
func downloadFile(ctx context.Context, url, destPath string, size int64, onProgress func(DownloadProgress)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d", resp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if size <= 0 {
		size = resp.ContentLength
	}

	buf := make([]byte, 32*1024)
	var downloaded int64
	lastReport := time.Now()

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			downloaded += int64(n)
			if onProgress != nil && time.Since(lastReport) > 200*time.Millisecond {
				pct := 0
				if size > 0 {
					pct = int(downloaded * 100 / size)
				}
				onProgress(DownloadProgress{
					Downloaded: downloaded,
					Total:      size,
					Percent:    pct,
					Path:       destPath,
				})
				lastReport = time.Now()
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return readErr
		}
	}

	if onProgress != nil {
		pct := 100
		if size > 0 && downloaded < size {
			pct = int(downloaded * 100 / size)
		}
		onProgress(DownloadProgress{
			Downloaded: downloaded,
			Total:      size,
			Percent:    pct,
			Path:       destPath,
		})
	}

	return nil
}

// verifyChecksum downloads the .sha256 file and compares it against the
// actual hash of the local archive.
func verifyChecksum(ctx context.Context, archivePath, sha256URL string) error {
	if sha256URL == "" {
		debuglog.Printf("[UPDATER] no SHA-256 URL, skipping verification")
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sha256URL, nil)
	if err != nil {
		return fmt.Errorf("sha256 request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sha256 download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sha256 http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return fmt.Errorf("sha256 read: %w", err)
	}

	// Format: "hash  filename" or just "hash"
	parts := strings.Fields(strings.TrimSpace(string(body)))
	if len(parts) == 0 {
		return fmt.Errorf("empty sha256 file")
	}
	expectedHash := strings.ToLower(parts[0])

	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actualHash := hex.EncodeToString(h.Sum(nil))

	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	debuglog.Printf("[UPDATER] SHA-256 verified: %s", actualHash[:16]+"…")
	return nil
}

// PrepareInstall extracts the downloaded archive and returns the path to the
// new binary. On Windows it also creates a batch script that will replace the
// running exe after it exits.
func PrepareInstall(archivePath string) (string, error) {
	cfgDir, err := appdir.ConfigDir()
	if err != nil {
		return "", err
	}
	updatesDir := filepath.Join(cfgDir, "updates")
	extractDir := filepath.Join(updatesDir, "extracted")
	os.RemoveAll(extractDir)
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return "", err
	}

	switch {
	case strings.HasSuffix(archivePath, ".zip"):
		if err := extractZip(archivePath, extractDir); err != nil {
			return "", fmt.Errorf("extract zip: %w", err)
		}
	case strings.HasSuffix(archivePath, ".tar.gz"):
		if err := extractTarGz(archivePath, extractDir); err != nil {
			return "", fmt.Errorf("extract tar.gz: %w", err)
		}
	default:
		return "", fmt.Errorf("unsupported archive format: %s", archivePath)
	}

	// Find the new binary
	newBinary := findBinary(extractDir)
	if newBinary == "" {
		return "", fmt.Errorf("no binary found in archive")
	}

	debuglog.Printf("[UPDATER] prepared: %s", newBinary)
	return newBinary, nil
}

// findBinary locates the CursorBridge executable in the extracted directory.
func findBinary(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		name := e.Name()
		if runtime.GOOS == "windows" {
			if strings.EqualFold(name, "CursorBridge.exe") {
				return filepath.Join(dir, name)
			}
		} else {
			if name == "CursorBridge" && !e.IsDir() {
				return filepath.Join(dir, name)
			}
		}
	}
	// Check one level deep (macOS .app bundle, tar structure)
	for _, e := range entries {
		if e.IsDir() {
			sub := filepath.Join(dir, e.Name())
			if p := findBinary(sub); p != "" {
				return p
			}
		}
	}
	return ""
}

// InstallAndRestart replaces the running binary with the new one and restarts.
// On Windows, this creates a batch script because the running exe is locked.
// On macOS/Linux, it uses a shell script.
func InstallAndRestart(newBinary string, quitFn func()) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get exe path: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolve exe path: %w", err)
	}

	cfgDir, err := appdir.ConfigDir()
	if err != nil {
		return err
	}
	updatesDir := filepath.Join(cfgDir, "updates")

	switch runtime.GOOS {
	case "windows":
		return installWindows(exe, newBinary, updatesDir, quitFn)
	default:
		return installUnix(exe, newBinary, updatesDir, quitFn)
	}
}

func installWindows(exe, newBinary, updatesDir string, quitFn func()) error {
	script := filepath.Join(updatesDir, "apply-update.bat")
	content := fmt.Sprintf(`@echo off
echo Applying update...
timeout /t 2 /nobreak >nul
:retry
copy /y "%s" "%s"
if errorlevel 1 (
    timeout /t 1 /nobreak >nul
    goto retry
)
start "" "%s"
del "%s"
del "%s"
`, newBinary, exe, exe, newBinary, script)

	if err := os.WriteFile(script, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write script: %w", err)
	}

	debuglog.Printf("[UPDATER] launching update script: %s", script)
	// Launch the script detached
	if err := startDetached("cmd.exe", "/c", script); err != nil {
		return fmt.Errorf("start script: %w", err)
	}

	if quitFn != nil {
		quitFn()
	}
	return nil
}

func installUnix(exe, newBinary, updatesDir string, quitFn func()) error {
	script := filepath.Join(updatesDir, "apply-update.sh")
	content := fmt.Sprintf(`#!/bin/sh
echo "Applying update..."
sleep 2
for i in 1 2 3 4 5; do
    cp -f "%s" "%s" && break
    sleep 1
done
chmod +x "%s"
nohup "%s" >/dev/null 2>&1 &
rm -f "%s"
rm -f "%s"
`, newBinary, exe, exe, exe, newBinary, script)

	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		return fmt.Errorf("write script: %w", err)
	}

	debuglog.Printf("[UPDATER] launching update script: %s", script)
	if err := startDetached("/bin/sh", script); err != nil {
		return fmt.Errorf("start script: %w", err)
	}

	if quitFn != nil {
		quitFn()
	}
	return nil
}

// startDetached launches a command detached from the current process.
func startDetached(name string, args ...string) error {
	// Use os/exec but detach the process so it survives our exit.
	// On Windows, we need Cmd.SysProcAttr with CREATE_NEW_PROCESS_GROUP.
	attr, err := detachedProcessAttr()
	if err != nil {
		return err
	}
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = attr
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}
