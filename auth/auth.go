package auth

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/hishantik/anilix/browser"
	"github.com/hishantik/anilix/config"
	"github.com/hishantik/anilix/curl"
	"github.com/hishantik/anilix/provider/anilist"
)

const (
	ClientID     = "42360"
	ClientSecret = "55OfLc1HUwI7TDkF4Clnr4QMt98HRxTmbqNW2ITY"
	CallbackPort = 9999
	RedirectURI  = "http://localhost:9999"
)

// Quiet suppresses stdout output when true (for TUI usage).
var Quiet bool

func info(format string, a ...interface{}) {
	if !Quiet {
		fmt.Printf(format, a...)
	}
}

func infoPrintln(a ...interface{}) {
	if !Quiet {
		fmt.Println(a...)
	}
}

// Login performs the OAuth2 Authorization Code Grant flow:
// starts a localhost callback server, opens the browser, waits for the code, exchanges it for a token.
func Login() error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", CallbackPort))
	if err != nil {
		return fmt.Errorf("port %d is already in use: %w", CallbackPort, err)
	}
	defer listener.Close()

	authURL := fmt.Sprintf(
		"https://anilist.co/api/v2/oauth/authorize?client_id=%s&redirect_uri=%s&response_type=code",
		ClientID, RedirectURI,
	)

	infoPrintln("Opening browser for AniList login...")
	if err := browser.Open(authURL); err != nil {
		info("Could not open browser automatically.\nPlease open this URL manually:\n\n  %s\n\n", authURL)
	}

	infoPrintln("Waiting for AniList authorization...")

	code, err := waitForCode(listener)
	if err != nil {
		return err
	}

	return exchangeCode(code)
}

// LoginManual performs the Authorization Code Grant flow without opening a browser.
func LoginManual() error {
	authURL := fmt.Sprintf(
		"https://anilist.co/api/v2/oauth/authorize?client_id=%s&redirect_uri=%s&response_type=code",
		ClientID, RedirectURI,
	)

	info("Open this URL in your browser:\n\n  %s\n\n", authURL)
	infoPrintln("After authorizing, you will be redirected to a URL like:")
	info("  %s/?code=XXXXXX\n", RedirectURI)
	infoPrintln()
	info("Paste the full redirect URL here: ")

	input := readLine()

	code, err := parseCodeFromURL(input)
	if err != nil {
		return err
	}

	return exchangeCode(code)
}

// Logout clears the stored AniList credentials.
func Logout() error {
	config.Set("anilist.token", "")
	config.Set("anilist.username", "")
	config.Set("anilist.user_id", 0)
	infoPrintln("Logged out of AniList.")
	return nil
}

// IsLoggedIn returns true if an AniList token is stored.
func IsLoggedIn() bool {
	return config.GetString("anilist.token") != ""
}

// GetToken returns the stored AniList access token.
func GetToken() string {
	return config.GetString("anilist.token")
}

// GetUsername returns the stored AniList username.
func GetUsername() string {
	return config.GetString("anilist.username")
}

// exchangeCode exchanges the authorization code for an access token via curl.
func exchangeCode(code string) error {
	tokenReq := map[string]interface{}{
		"grant_type":    "authorization_code",
		"client_id":     ClientID,
		"client_secret": ClientSecret,
		"redirect_uri":  RedirectURI,
		"code":          code,
	}

	body, err := json.Marshal(tokenReq)
	if err != nil {
		return fmt.Errorf("failed to marshal token request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := curl.Post(ctx, "https://anilist.co/api/v2/oauth/token", nil, string(body))
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
		Message     string `json:"message"`
	}
	if err := json.Unmarshal([]byte(resp), &tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	if tokenResp.Error != "" {
		return fmt.Errorf("token exchange error: %s - %s", tokenResp.Error, tokenResp.Message)
	}

	if tokenResp.AccessToken == "" {
		return fmt.Errorf("no access_token in response: %s", resp)
	}

	return finishLogin(tokenResp.AccessToken)
}

func finishLogin(token string) error {
	config.Set("anilist.token", token)

	client := anilist.NewAuthenticatedClient(token)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	username, userID, err := client.GetViewer(ctx)
	if err != nil {
		log.Printf("[anilix] warning: token saved but could not fetch user info: %v\n", err)
		infoPrintln("Token saved, but could not verify login. It may be invalid.")
		return nil
	}

	config.Set("anilist.username", username)
	config.Set("anilist.user_id", userID)
	info("Logged in to AniList as %s\n", username)
	return nil
}

// waitForCode accepts one HTTP connection, extracts the authorization code, serves a success page.
func waitForCode(listener net.Listener) (string, error) {
	listener.(*net.TCPListener).SetDeadline(time.Now().Add(2 * time.Minute))

	conn, err := listener.Accept()
	if err != nil {
		return "", fmt.Errorf("timed out waiting for authorization: %w", err)
	}
	defer conn.Close()

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return "", fmt.Errorf("failed to read callback request: %w", err)
	}

	requestLine := string(buf[:n])

	code := parseCodeFromRequest(requestLine)
	if code == "" {
		if strings.Contains(requestLine, "error=") {
			conn.Write([]byte(successPage("Authorization was denied or failed. Please try again.")))
			return "", fmt.Errorf("authorization was denied")
		}
		conn.Write([]byte(successPage("No authorization code received. Please try again.")))
		return "", fmt.Errorf("no authorization code in callback")
	}

	conn.Write([]byte(successPage("Login successful! You may close this tab.")))
	return code, nil
}

func successPage(message string) string {
	body := fmt.Sprintf("<!DOCTYPE html><html><head><title>Anilix</title></head><body><p>%s</p></body></html>", message)
	return fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
		len(body), body)
}

func parseCodeFromRequest(requestLine string) string {
	lines := strings.SplitN(requestLine, "\r\n", 2)
	if len(lines) == 0 {
		return ""
	}
	parts := strings.SplitN(lines[0], " ", 3)
	if len(parts) < 2 {
		return ""
	}
	path := parts[1]
	idx := strings.Index(path, "?")
	if idx == -1 {
		return ""
	}
	query := path[idx+1:]
	for _, param := range strings.Split(query, "&") {
		kv := strings.SplitN(param, "=", 2)
		if len(kv) == 2 && kv[0] == "code" {
			return kv[1]
		}
	}
	return ""
}

func parseCodeFromURL(url string) (string, error) {
	idx := strings.Index(url, "?")
	if idx == -1 {
		return "", fmt.Errorf("no query string found in URL — expected ?code=...")
	}
	query := url[idx+1:]
	for _, param := range strings.Split(query, "&") {
		kv := strings.SplitN(param, "=", 2)
		if len(kv) == 2 && kv[0] == "code" {
			return kv[1], nil
		}
	}
	return "", fmt.Errorf("no code found in URL")
}

func readLine() string {
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return scanner.Text()
	}
	return ""
}
