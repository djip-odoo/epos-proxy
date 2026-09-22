// Package update checks the app's GitHub releases for a newer build and allows
// the user to download and apply it in place.
package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"epos-proxy/internal/logger"
)

// GitHub repository hosting the release assets checked by the app.
const (
	RepoOwner = "djip-odoo"
	RepoName  = "epos-proxy"

	assetNameLinux   = "epos-proxy-linux64"
	assetNameWindows = "epos-proxy-win64-installer.exe"

	requestUA   = "epos-proxy-updater"
	apiTimeout  = 15 * time.Second
	downloadBuf = 64 * 1024
)

// Version is the version of the running binary. It is injected at build time
// from the release tag via:
//
//	go build -ldflags "-X epos-proxy/internal/update.Version=v1.2.3"
//
// The "dev" fallback is always treated as older than any published release.
var Version = "dev"

var (
	// apiBaseURL is a variable so tests can point the check at a fake server.
	apiBaseURL = "https://api.github.com"

	ErrNoReleases = errors.New("no published releases found")
	ErrNoMatchOS  = errors.New("no release asset for this operating system")
)

// GitHubAsset is one downloadable file attached to a GitHub release.
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// GitHubRelease is the subset of a GitHub release the updater needs.
type GitHubRelease struct {
	TagName string        `json:"tag_name"`
	HTMLURL string        `json:"html_url"`
	Body    string        `json:"body"`
	Assets  []GitHubAsset `json:"assets"`
}

// Info is the result of checking for updates, intended for the UI.
type Info struct {
	CheckOK        bool   `json:"checkOk"`
	Available      bool   `json:"available"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	DownloadURL    string `json:"downloadUrl"`
	AssetName      string `json:"assetName"`
	AssetSize      int64  `json:"assetSize"`
	Notes          string `json:"notes"`
	Error          string `json:"error,omitempty"`
}

// Check queries the latest release and reports whether an upgrade applies to
// the current OS. It never returns an error: failures are folded into the
// returned Info so the caller can show them inline.
func Check() Info {
	return CheckGoOS(runtime.GOOS)
}

// CheckGoOS is Check for an explicit target OS so the logic is testable.
func CheckGoOS(goos string) Info {
	info := Info{
		CurrentVersion: Version,
	}
	client := &http.Client{Timeout: apiTimeout}
	rel, err := fetchRelease(client, apiBaseURL+"/repos/"+RepoOwner+"/"+RepoName+"/releases/latest")
	if err != nil {
		info.Error = err.Error()
		return info
	}

	info.LatestVersion = rel.TagName
	info.Notes = trimNotes(rel.Body)

	asset, err := assetForOS(rel, goos)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	info.DownloadURL = asset.BrowserDownloadURL
	info.AssetName = asset.Name
	info.AssetSize = asset.Size
	info.CheckOK = true

	info.Available = compareVersion(Version, rel.TagName) < 0
	return info
}

// fetchRelease GETs a GitHub release JSON document. The client is passed in so
// tests can substitute a fake server; callers should set a sane timeout.
func fetchRelease(client *http.Client, url string) (*GitHubRelease, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("release check failed: %w", err)
	}
	req.Header.Set("User-Agent", requestUA+"/"+Version)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("release check failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrNoReleases
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("release check failed: github returned %d", resp.StatusCode)
	}

	var rel GitHubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel); err != nil {
		return nil, fmt.Errorf("release check failed: %w", err)
	}
	return &rel, nil
}

// assetForOS picks the correct release asset for the requested OS, matching
// what the CI release workflow publishes.
func assetForOS(rel *GitHubRelease, goos string) (GitHubAsset, error) {
	var want string
	switch goos {
	case "windows":
		want = assetNameWindows
	case "linux":
		want = assetNameLinux
	default:
		return GitHubAsset{}, fmt.Errorf("no build is published for %q: %w", goos, ErrNoMatchOS)
	}
	for _, a := range rel.Assets {
		if a.Name == want {
			return a, nil
		}
	}
	return GitHubAsset{}, fmt.Errorf("release %q has no %s asset: %w", rel.TagName, want, ErrNoMatchOS)
}

// trimNotes keeps the release notes short for the update banner.
func trimNotes(body string) string {
	body = strings.TrimSpace(body)
	if len(body) > 500 {
		return body[:500] + "…"
	}
	return body
}

// ProgressFunc reports the downloaded and total byte counts during a download.
type ProgressFunc func(downloaded, total int64)

// Download streams the asset at url into dir under filename and returns the
// local path. A non-nil progress callback is invoked as bytes arrive.
func Download(url, filename, dir string, progress ProgressFunc) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}

	client := &http.Client{Timeout: 0} // large binaries; no global deadline
	logger.Infof("Downloading update from %s", url)
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return "", fmt.Errorf("download failed: github returned %d", resp.StatusCode)
	}

	dest, err := os.Create(filepath.Join(dir, filename))
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}

	total := resp.ContentLength
	written, copyErr := copyWithProgress(dest, resp.Body, total, progress)
	closeErr := dest.Close()
	if copyErr != nil {
		_ = os.Remove(dest.Name())
		return "", fmt.Errorf("download failed: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(dest.Name())
		return "", fmt.Errorf("download failed: %w", closeErr)
	}
	if total > 0 && written != total {
		_ = os.Remove(dest.Name())
		return "", fmt.Errorf("download incomplete: got %d of %d bytes", written, total)
	}
	if total > 0 && progress != nil {
		progress(total, total)
	}
	logger.Infof("Update downloaded (%d bytes) to %s", written, dest.Name())
	return dest.Name(), nil
}

func copyWithProgress(dst io.Writer, src io.Reader, total int64, progress ProgressFunc) (int64, error) {
	if progress == nil {
		return io.Copy(dst, src)
	}
	buf := make([]byte, downloadBuf)
	var written int64
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, err := dst.Write(buf[:n]); err != nil {
				return written, err
			}
			written += int64(n)
			progress(written, total)
		}
		switch readErr {
		case nil:
		case io.EOF:
			return written, nil
		default:
			return written, readErr
		}
	}
}

// compareVersion compares two version tags such as "v1.2.3" or "1.2.3-rc.1"
// and returns -1, 0 or 1. Strings that are not version tags (for example the
// "dev" default) sort before any tag with -1/1 semantics.
func compareVersion(a, b string) int {
	va, errA := parseVersion(a)
	vb, errB := parseVersion(b)
	if errA != nil || errB != nil {
		switch {
		case errA != nil && errB != nil:
			return strings.Compare(a, b)
		case errA != nil:
			return -1 // unknown/"dev" is always older than a tag
		default:
			return 1
		}
	}

	if va.maj != vb.maj {
		return compareInt(va.maj, vb.maj)
	}
	if va.min != vb.min {
		return compareInt(va.min, vb.min)
	}
	if va.pat != vb.pat {
		return compareInt(va.pat, vb.pat)
	}

	switch {
	case va.pre == "" && vb.pre != "":
		return 1 // release > pre-release on the same version
	case va.pre != "" && vb.pre == "":
		return -1
	default:
		return strings.Compare(va.pre, vb.pre)
	}
}

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

type parsedVersion struct {
	maj, min, pat int
	pre           string
}

// parseVersion splits "v1.2.3-rc.1" into numeric core parts and a pre-release
// component. It errors when the core is not numeric.
func parseVersion(s string) (parsedVersion, error) {
	s = strings.TrimSpace(strings.TrimPrefix(s, "v"))
	if s == "" {
		return parsedVersion{}, errors.New("empty version")
	}

	var pre string
	if i := strings.IndexByte(s, '-'); i >= 0 {
		pre = s[i+1:]
		s = s[:i]
	}

	var nums []int
	for _, part := range strings.Split(s, ".") {
		n, err := strconv.Atoi(part)
		if err != nil {
			return parsedVersion{}, fmt.Errorf("not a version tag: %q", s)
		}
		nums = append(nums, n)
	}

	v := parsedVersion{pre: pre}
	if len(nums) > 0 {
		v.maj = nums[0]
	}
	if len(nums) > 1 {
		v.min = nums[1]
	}
	if len(nums) > 2 {
		v.pat = nums[2]
	}
	return v, nil
}
