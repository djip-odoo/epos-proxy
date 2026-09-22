package update

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"v1.2.3", "v1.2.4", -1},
		{"v1.2.4", "v1.2.3", 1},
		{"1.2.3", "1.2.3", 0},
		{"v1.2.3", "v2.0.0", -1},
		{"v2.0.0", "v10.0.0", -1},
		{"v1.2.3", "v1.2", 1},
		{"v1.2", "v1.2.0", 0},
		{"v1.2.3-rc.1", "v1.2.3", -1}, // pre-release is older
		{"v1.2.3", "v1.2.3-rc.1", 1},  // release beats pre-release
		{"v2.0.0-rc.1", "v2.0.0", -1},
		{"dev", "v1.0.0", -1}, // unknown/"dev" is older than any tag
		{"", "v1.0.0", -1},
		{"dev", "dev", 0},
		{"v1.2.3", "dev", 1},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s vs %s", tc.a, tc.b), func(t *testing.T) {
			if got := compareVersion(tc.a, tc.b); got != tc.want {
				t.Errorf("compareVersion(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestAssetForOS(t *testing.T) {
	rel := &GitHubRelease{
		TagName: "v1.2.3",
		Assets: []GitHubAsset{
			{Name: assetNameLinux, BrowserDownloadURL: "https://x/linux"},
			{Name: assetNameWindows, BrowserDownloadURL: "https://x/win"},
		},
	}

	got, err := assetForOS(rel, "linux")
	if err != nil {
		t.Fatalf("linux: unexpected error %v", err)
	}
	if got.BrowserDownloadURL != "https://x/linux" {
		t.Errorf("linux: got URL %q", got.BrowserDownloadURL)
	}

	got, err = assetForOS(rel, "windows")
	if err != nil {
		t.Fatalf("windows: unexpected error %v", err)
	}
	if got.BrowserDownloadURL != "https://x/win" {
		t.Errorf("windows: got URL %q", got.BrowserDownloadURL)
	}

	if _, err = assetForOS(rel, "darwin"); !errors.Is(err, ErrNoMatchOS) {
		t.Errorf("darwin: got err %v, want ErrNoMatchOS", err)
	}
}

func TestAssetForOSMissingAsset(t *testing.T) {
	rel := &GitHubRelease{TagName: "v1.2.3", Assets: []GitHubAsset{}}
	if _, err := assetForOS(rel, "linux"); !errors.Is(err, ErrNoMatchOS) {
		t.Errorf("missing asset: got err %v, want ErrNoMatchOS", err)
	}
}

func TestFetchRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); !strings.HasPrefix(ua, requestUA) {
			t.Errorf("unexpected User-Agent %q", ua)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"tag_name": "v1.2.3",
			"html_url": "https://github.com/odoo/obox-app/releases/tag/v1.2.3",
			"body": "fixes",
			"assets": [
				{"name": "epos-proxy-linux64", "browser_download_url": "https://x/linux", "size": 123}
			]
		}`)
	}))
	defer server.Close()

	rel, err := fetchRelease(server.Client(), server.URL)
	if err != nil {
		t.Fatalf("fetchRelease: %v", err)
	}
	if rel.TagName != "v1.2.3" {
		t.Errorf("TagName = %q", rel.TagName)
	}
	if len(rel.Assets) != 1 || rel.Assets[0].Size != 123 {
		t.Errorf("unexpected assets: %+v", rel.Assets)
	}
}

func TestFetchReleaseNotFound(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	if _, err := fetchRelease(server.Client(), server.URL); !errors.Is(err, ErrNoReleases) {
		t.Errorf("got err %v, want ErrNoReleases", err)
	}
}

func TestFetchReleaseBadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	if _, err := fetchRelease(server.Client(), server.URL); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCheckGoOS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"tag_name": "v1.2.3",
			"body": "fixes",
			"assets": [
				{"name": "epos-proxy-linux64", "browser_download_url": "https://x/linux", "size": 123}
			]
		}`)
	}))
	defer server.Close()

	apiURL := apiBaseURL
	defer func() { apiBaseURL = apiURL }()
	apiBaseURL = server.URL

	oldVersion := Version
	defer func() { Version = oldVersion }()

	Version = "dev"
	info := CheckGoOS("linux")
	if !info.CheckOK || !info.Available {
		t.Errorf("dev build should report update available: %+v", info.Error)
	}

	Version = "v1.2.3"
	info = CheckGoOS("linux")
	if !info.CheckOK || info.Available {
		t.Errorf("same version should not report update available: %+v", info)
	}
	if info.LatestVersion != "v1.2.3" || info.AssetName != assetNameLinux {
		t.Errorf("unexpected info: %+v", info)
	}

	info = CheckGoOS("darwin")
	if info.CheckOK {
		t.Errorf("darwin has no asset, CheckOK should be false: %+v", info)
	}
}

func TestDownload(t *testing.T) {
	payload := []byte("update-binary-payload")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	dir := t.TempDir()
	var last int64
	path, err := Download(server.URL, "pkg.bin", dir, func(d, total int64) {
		last = d
		if d > total || total != int64(len(payload)) {
			t.Errorf("bad progress d=%d total=%d", d, total)
		}
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if got := readFile(t, path); string(got) != string(payload) {
		t.Errorf("downloaded content mismatch: %q", got)
	}
	if last != int64(len(payload)) {
		t.Errorf("progress never reached total: got %d", last)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}
