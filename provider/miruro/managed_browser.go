package miruro

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

var ErrBrowserUnavailable = errors.New("supported Chromium browser unavailable")

type managedBrowserFetcher struct {
	executable string
	profileDir string

	mu          sync.Mutex
	allocCancel context.CancelFunc
	ctxCancel   context.CancelFunc
	ctx         context.Context
	origin      string
}

func NewManagedBrowserTransport(profileDir string, origins []string) (Transport, error) {
	executable, err := findBrowserExecutable()
	if err != nil {
		return nil, err
	}
	fetcher := &managedBrowserFetcher{executable: executable, profileDir: profileDir}
	return NewBrowserTransport(fetcher, origins), nil
}

func (f *managedBrowserFetcher) Fetch(ctx context.Context, origin, relativeURL string) (browserResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.ensureStarted(origin); err != nil {
		return browserResponse{}, err
	}
	if f.origin != origin {
		if err := chromedp.Run(f.ctx, chromedp.Navigate(origin), chromedp.WaitReady("body", chromedp.ByQuery)); err != nil {
			return browserResponse{}, fmt.Errorf("navigate Miruro browser: %w", err)
		}
		f.origin = origin
	}

	type evaluationResult struct {
		Status     int    `json:"status"`
		Body       string `json:"body"`
		Obfuscated string `json:"obfuscated"`
	}
	var result evaluationResult
	expression := fmt.Sprintf(`(async () => {
		const response = await fetch(%q, {credentials: "include"});
		return {
			status: response.status,
			body: await response.text(),
			obfuscated: response.headers.get("x-obfuscated") || ""
		};
	})()`, relativeURL)

	done := make(chan error, 1)
	go func() {
		done <- chromedp.Run(f.ctx, chromedp.Evaluate(expression, &result, chromedp.EvalAsValue))
	}()
	select {
	case <-ctx.Done():
		return browserResponse{}, ctx.Err()
	case err := <-done:
		if err != nil {
			return browserResponse{}, fmt.Errorf("fetch Miruro pipe in browser: %w", err)
		}
	}
	return browserResponse{Status: result.Status, Body: result.Body, Obfuscated: result.Obfuscated}, nil
}

func (f *managedBrowserFetcher) ensureStarted(origin string) error {
	if f.ctx != nil {
		return nil
	}
	if err := os.MkdirAll(f.profileDir, 0o700); err != nil {
		return fmt.Errorf("create Miruro browser profile: %w", err)
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(f.executable),
		chromedp.UserDataDir(f.profileDir),
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	browserCtx, ctxCancel := chromedp.NewContext(allocCtx)
	f.allocCancel = allocCancel
	f.ctxCancel = ctxCancel
	f.ctx = browserCtx

	startCtx, cancel := context.WithTimeout(browserCtx, 120*time.Second)
	defer cancel()
	if err := chromedp.Run(startCtx, chromedp.Navigate(origin), chromedp.WaitReady("body", chromedp.ByQuery)); err != nil {
		f.Close()
		return fmt.Errorf("start Miruro browser (complete any visible verification): %w", err)
	}
	f.origin = origin
	return nil
}

func (f *managedBrowserFetcher) Close() error {
	if f.ctxCancel != nil {
		f.ctxCancel()
	}
	if f.allocCancel != nil {
		f.allocCancel()
	}
	f.ctx = nil
	f.ctxCancel = nil
	f.allocCancel = nil
	f.origin = ""
	return nil
}

func findBrowserExecutable() (string, error) {
	programFiles := os.Getenv("ProgramFiles")
	programFilesX86 := os.Getenv("ProgramFiles(x86)")
	localAppData := os.Getenv("LOCALAPPDATA")
	for _, candidate := range browserExecutableCandidates(runtime.GOOS, programFiles, programFilesX86, localAppData) {
		if candidate == "" {
			continue
		}
		if strings.ContainsRune(candidate, filepath.Separator) || filepath.IsAbs(candidate) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, nil
			}
			continue
		}
		if path, err := findExecutableOnPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", ErrBrowserUnavailable
}

func findExecutableOnPath(name string) (string, error) {
	pathExt := ""
	if runtime.GOOS == "windows" {
		pathExt = ".exe"
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		candidate := filepath.Join(dir, name+pathExt)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", os.ErrNotExist
}

func browserExecutableCandidates(goos, programFiles, programFilesX86, localAppData string) []string {
	switch goos {
	case "windows":
		return []string{
			filepath.Join(programFiles, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFilesX86, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFiles, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(programFilesX86, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(localAppData, "Microsoft", "Edge", "Application", "msedge.exe"),
		}
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	case "linux":
		return []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge"}
	default:
		return nil
	}
}
