package browser

import (
	"fmt"
	"os/exec"
	"runtime"
)

func Open(rawURL string) error {
	if runtime.GOOS == "android" {
		if err := exec.Command("termux-open-url", rawURL).Start(); err == nil {
			return nil
		}
		return exec.Command("am", "start", "-a", "android.intent.action.VIEW", "-d", rawURL).Start()
	}
	name, args, err := commandForOS(runtime.GOOS, rawURL)
	if err != nil {
		return err
	}
	return exec.Command(name, args...).Start()
}

func commandForOS(goos, rawURL string) (string, []string, error) {
	switch goos {
	case "linux":
		return "xdg-open", []string{rawURL}, nil
	case "darwin":
		return "open", []string{rawURL}, nil
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", rawURL}, nil
	default:
		return "", nil, fmt.Errorf("unsupported platform: %s", goos)
	}
}
