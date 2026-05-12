package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ANSI colors (disabled when not a tty)
var (
	yellow = "\033[33m"
	green  = "\033[32m"
	cyan   = "\033[36m"
	bold   = "\033[1m"
	red    = "\033[31m"
	dim    = "\033[2m"
	reset  = "\033[0m"
)

var noColor bool

func init() {
	noColor = os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" || !isTTY()
	if noColor {
		yellow, green, cyan, bold, red, dim, reset = "", "", "", "", "", "", ""
	}
}

func isTTY() bool {
	_, err := exec.Command("tty", "-s").CombinedOutput()
	return err == nil
}

func c(color, s string) string {
	if noColor {
		return s
	}
	return color + s + reset
}

// Config

type Config struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

var configDir = func() string {
	u, _ := user.Current()
	dir := u.HomeDir
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		dir = xdg
	} else {
		dir = filepath.Join(dir, ".config")
	}
	return filepath.Join(dir, "share-home")
}

func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

func loadConfig() Config {
	var cfg Config
	if url := os.Getenv("SHARE_HOME_URL"); url != "" {
		cfg.URL = url
	}
	if tok := os.Getenv("SHARE_HOME_TOKEN"); tok != "" {
		cfg.Token = tok
	}
	if cfg.URL == "" {
		data, err := os.ReadFile(configPath())
		if err == nil {
			json.Unmarshal(data, &cfg)
		}
	}
	// env vars always override
	if os.Getenv("SHARE_HOME_URL") != "" {
		cfg.URL = os.Getenv("SHARE_HOME_URL")
	}
	if os.Getenv("SHARE_HOME_TOKEN") != "" {
		cfg.Token = os.Getenv("SHARE_HOME_TOKEN")
	}
	return cfg
}

func saveConfig(cfg Config) error {
	p := configPath()
	os.MkdirAll(configDir(), 0755)
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(p, data, 0644)
}

// HTTP helpers

func do(cfg Config, method, path string, body []byte, contentType string) (*http.Response, error) {
	fullURL := cfg.URL + path
	if cfg.Token != "" {
		if !strings.Contains(fullURL, "?") {
			fullURL += "?"
		} else {
			fullURL += "&"
		}
		fullURL += "token=" + url.QueryEscape(cfg.Token)
	}
	req, err := http.NewRequest(method, fullURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return http.DefaultClient.Do(req)
}

func doJSON(cfg Config, method, path string, payload interface{}) (*http.Response, error) {
	data, _ := json.Marshal(payload)
	return do(cfg, method, path, data, "application/json")
}

func doUpload(cfg Config, filename string, data []byte, expires string) (*http.Response, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", filepath.Base(filename))
	fw.Write(data)
	if expires != "" {
		w.WriteField("expires_at", expires)
	}
	w.Close()

	fullURL := cfg.URL + "/api/upload"
	if cfg.Token != "" {
		fullURL += "?token=" + url.QueryEscape(cfg.Token)
	}
	req, _ := http.NewRequest("POST", fullURL, &buf)
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	return http.DefaultClient.Do(req)
}

// Output helpers

func printResult(cfg Config, link, id string, jsonOut bool) {
	if jsonOut {
		json.NewEncoder(os.Stdout).Encode(map[string]string{
			"id":   id,
			"link": link,
		})
		return
	}
	fmt.Printf("%s %s\n%s %s%s\n", c(yellow, "link:"), link, c(cyan, "id:"), id, c(dim, fmt.Sprintf("  (run %s)", c(bold, "share-home open "+id))))
}

// Commands

func cmdUpload(cfg Config, args []string, jsonOut bool) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: share-home upload <file> [--expires 1h|1d|1w]\n")
		os.Exit(1)
	}

	var expires, filename string
	for i := 0; i < len(args); i++ {
		if args[i] == "--expires" && i+1 < len(args) {
			expires = args[i+1]
			i++
		} else if !strings.HasPrefix(args[i], "-") {
			filename = args[i]
		}
	}
	if filename == "" {
		fmt.Fprintf(os.Stderr, "usage: share-home upload <file> [--expires 1h|1d|1w]\n")
		os.Exit(1)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %s\n", c(red, "error:"), err)
		os.Exit(1)
	}

	if !jsonOut {
		fmt.Fprintf(os.Stderr, "%s %s (%s)\n", c(green, "uploading"), filepath.Base(filename), fmtSize(int64(len(data))))
	}

	resp, err := doUpload(cfg, filename, data, expires)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %s\n", c(red, "error:"), err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 201 {
		fmt.Fprintf(os.Stderr, "%s %s\n", c(red, "upload failed:"), string(body))
		os.Exit(1)
	}

	var result struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		DownloadURL string `json:"download_url"`
	}
	json.Unmarshal(body, &result)
	link := cfg.URL + result.DownloadURL
	printResult(cfg, link, result.ID, jsonOut)
}

func cmdPaste(cfg Config, args []string, jsonOut bool, fromURL string) {
	var text string
	if fromURL != "" {
		text = fromURL
	} else if len(args) == 0 {
		// check stdin
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s %s\n", c(red, "error:"), err)
			os.Exit(1)
		}
		text = strings.TrimSuffix(string(data), "\n")
	} else {
		text = strings.Join(args, " ")
	}

	if text == "" {
		fmt.Fprintf(os.Stderr, "%s no text provided\n", c(red, "error:"))
		os.Exit(1)
	}

	resp, err := doJSON(cfg, "POST", "/api/clipboard", map[string]string{
		"type":    "text",
		"content": text,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %s\n", c(red, "error:"), err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 201 {
		fmt.Fprintf(os.Stderr, "%s %s\n", c(red, "paste failed:"), string(body))
		os.Exit(1)
	}

	var result struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	json.Unmarshal(body, &result)
	link := cfg.URL + result.URL
	printResult(cfg, link, result.ID, jsonOut)
}

func cmdURL(cfg Config, args []string, jsonOut bool) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: share-home url <url>\n")
		os.Exit(1)
	}
	target := args[0]

	resp, err := doJSON(cfg, "POST", "/api/url", map[string]string{
		"url": target,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %s\n", c(red, "error:"), err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 201 {
		fmt.Fprintf(os.Stderr, "%s %s\n", c(red, "shorten failed:"), string(body))
		os.Exit(1)
	}

	var result struct {
		Code     string `json:"code"`
		ShortURL string `json:"short_url"`
	}
	json.Unmarshal(body, &result)
	link := cfg.URL + result.ShortURL
	if jsonOut {
		json.NewEncoder(os.Stdout).Encode(map[string]string{
			"code":  result.Code,
			"link":  link,
			"from":  target,
		})
		return
	}
	fmt.Printf("%s %s\n%s %s\n%s %s\n", c(yellow, "short:"), link, c(cyan, "from:"), target, c(cyan, "id:"), result.Code)
}

type fileEntry struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	MIME    string `json:"mime"`
	URL     string `json:"url"`
	DL      int    `json:"downloads"`
	Expires string `json:"expires_at"`
}

type clipEntry struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

type urlEntry struct {
	Code     string `json:"code"`
	LongURL  string `json:"long_url"`
	ShortURL string `json:"short_url"`
}

func cmdList(cfg Config, args []string, jsonOut bool) {
	var filter string
	if len(args) > 0 {
		filter = args[0]
	}

	// Fetch all in parallel
	type fetch struct {
		files []fileEntry
		clips []clipEntry
		urls  []urlEntry
	}
	out := fetch{}

	ch := make(chan struct{}, 3)
	go func() {
		if filter == "" || filter == "files" {
			resp, err := do(cfg, "GET", "/api/files", nil, "")
			if err == nil && resp.StatusCode == 200 {
				json.NewDecoder(resp.Body).Decode(&out.files)
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
		ch <- struct{}{}
	}()
	go func() {
		if filter == "" || filter == "clip" || filter == "clipboard" {
			resp, err := do(cfg, "GET", "/api/clipboard", nil, "")
			if err == nil && resp.StatusCode == 200 {
				json.NewDecoder(resp.Body).Decode(&out.clips)
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
		ch <- struct{}{}
	}()
	go func() {
		if filter == "" || filter == "urls" || filter == "links" {
			resp, err := do(cfg, "GET", "/api/urls", nil, "")
			if err == nil && resp.StatusCode == 200 {
				json.NewDecoder(resp.Body).Decode(&out.urls)
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
		ch <- struct{}{}
	}()
	<-ch
	<-ch
	<-ch

	if jsonOut {
		json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
			"files":     out.files,
			"clipboard": out.clips,
			"links":     out.urls,
		})
		return
	}

	if filter == "" || filter == "files" {
		if len(out.files) == 0 {
			fmt.Printf("%s no files\n", c(dim, "files:"))
		} else {
			fmt.Printf("%s %d file(s)\n", c(green, "files:"), len(out.files))
			for _, f := range out.files {
				icon := fileIcon(f.MIME)
				id := c(bold, f.ID[:8])
				expires := ""
				if f.Expires != "" {
					expires = c(dim, " ⏰ "+f.Expires[:10])
				}
				downloads := ""
				if f.DL > 0 {
					downloads = c(cyan, fmt.Sprintf(" ⬇%d", f.DL))
				}
				fmt.Printf("  %s %s  %s %s %s\n", id, icon, c(yellow, f.Name), fmtSize(f.Size), downloads+expires)
			}
		}
		fmt.Println()
	}

	if filter == "" || filter == "clip" || filter == "clipboard" {
		if len(out.clips) == 0 {
			fmt.Printf("%s no clipboard entries\n", c(dim, "clipboard:"))
		} else {
			fmt.Printf("%s %d item(s)\n", c(green, "clipboard:"), len(out.clips))
			for _, c2 := range out.clips {
				ico := "txt"
				if c2.Type == "image" {
					ico = "img"
				}
				fmt.Printf("  %-8s %s %s\n", c(bold, c2.ID[:8]), c(yellow, ico), c2.Type)
			}
		}
		fmt.Println()
	}

	if filter == "" || filter == "urls" || filter == "links" {
		if len(out.urls) == 0 {
			fmt.Printf("%s no links\n", c(dim, "links:"))
		} else {
			fmt.Printf("%s %d link(s)\n", c(green, "links:"), len(out.urls))
			for _, u := range out.urls {
				fmt.Printf("  %s  %s %s\n", c(bold, u.Code), c(yellow, u.ShortURL), c(dim, u.LongURL))
			}
		}
	}
}

func cmdGet(cfg Config, args []string, jsonOut bool) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: share-home get <id>\n")
		os.Exit(1)
	}
	id := args[0]

	// Try file first
	resp, err := do(cfg, "GET", "/api/download/"+id, nil, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %s\n", c(red, "error:"), err)
		os.Exit(1)
	}

	if resp.StatusCode == 200 {
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		filename := extractFilename(resp.Header.Get("Content-Disposition"), id)
		if err := os.WriteFile(filename, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "%s %s\n", c(red, "error saving:"), err)
			os.Exit(1)
		}
		if jsonOut {
			json.NewEncoder(os.Stdout).Encode(map[string]string{
				"file":  filename,
				"bytes": fmt.Sprint(len(data)),
			})
		} else {
			fmt.Printf("%s %s (%s)\n", c(green, "saved:"), filename, fmtSize(int64(len(data))))
		}
		return
	}
	resp.Body.Close()

	// Try clipboard
	resp, err = do(cfg, "GET", "/api/clipboard/"+id, nil, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %s\n", c(red, "error:"), err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		data, _ := io.ReadAll(resp.Body)
		if jsonOut {
			json.NewEncoder(os.Stdout).Encode(map[string]string{
				"type": resp.Header.Get("Content-Type"),
				"text": string(data),
			})
		} else {
			fmt.Printf("%s (text/%d bytes)\n%s\n", c(green, "clipboard"), len(data), string(data))
		}
		return
	}

	fmt.Fprintf(os.Stderr, "%s item not found (%s)\n", c(red, "error:"), id)
	os.Exit(1)
}

func cmdOpen(cfg Config, args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: share-home open <id>\n")
		os.Exit(1)
	}
	id := args[0]

	// figure out what type it is
	link := ""

	// try files
	resp, err := do(cfg, "GET", "/api/files", nil, "")
	if err == nil {
		var files []fileEntry
		json.NewDecoder(resp.Body).Decode(&files)
		resp.Body.Close()
		for _, f := range files {
			if f.ID == id || strings.HasPrefix(f.ID, id) {
				link = cfg.URL + "/api/download/" + f.ID
				break
			}
		}
	}

	if link == "" {
		resp, err = do(cfg, "GET", "/api/clipboard", nil, "")
		if err == nil {
			var clips []clipEntry
			json.NewDecoder(resp.Body).Decode(&clips)
			resp.Body.Close()
			for _, c2 := range clips {
				if c2.ID == id || strings.HasPrefix(c2.ID, id) {
					link = cfg.URL + "/api/clipboard/" + c2.ID
					break
				}
			}
		}
	}

	if link == "" {
		resp, err = do(cfg, "GET", "/api/urls", nil, "")
		if err == nil {
			var urls []urlEntry
			json.NewDecoder(resp.Body).Decode(&urls)
			resp.Body.Close()
			for _, u := range urls {
				if u.Code == id || strings.HasPrefix(u.Code, id) {
					link = cfg.URL + u.ShortURL
					break
				}
			}
		}
	}

	if link == "" {
		fmt.Fprintf(os.Stderr, "%s not found: %s\n", c(red, "error:"), id)
		os.Exit(1)
	}

	fmt.Printf("opening: %s\n", c(yellow, link))
	openBrowser(link)
}

func cmdCopy(cfg Config, args []string, jsonOut bool) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: share-home copy <id>\n")
		os.Exit(1)
	}
	id := args[0]

	link := findLink(cfg, id)
	if link == "" {
		fmt.Fprintf(os.Stderr, "%s not found: %s\n", c(red, "error:"), id)
		os.Exit(1)
	}

	if err := copyToClipboard(link); err != nil {
		if jsonOut {
			json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error(), "link": link})
		} else {
			fmt.Printf("%s clipboard unavailable, copy manually:\n%s\n", c(yellow, "warning:"), link)
		}
		return
	}
	if jsonOut {
		json.NewEncoder(os.Stdout).Encode(map[string]string{"copied": link})
	} else {
		fmt.Printf("%s %s\n", c(green, "copied:"), link)
	}
}

func cmdCat(cfg Config, args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: share-home cat <id>\n")
		os.Exit(1)
	}
	id := args[0]

	resp, err := do(cfg, "GET", "/api/clipboard/"+id, nil, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %s\n", c(red, "error:"), err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "%s %s\n", c(red, "cat failed:"), string(body))
		os.Exit(1)
	}
	io.Copy(os.Stdout, resp.Body)
}

func cmdDelete(cfg Config, args []string, jsonOut bool) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: share-home delete <id>\n")
		os.Exit(1)
	}
	id := args[0]

	paths := []string{
		"/api/files/" + id,
		"/api/clipboard/" + id,
		"/api/urls/" + id,
	}

	for _, p := range paths {
		resp, err := do(cfg, "DELETE", p, nil, "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s %s\n", c(red, "error:"), err)
			os.Exit(1)
		}
		if resp.StatusCode == 200 || resp.StatusCode == 204 {
			resp.Body.Close()
			if jsonOut {
				json.NewEncoder(os.Stdout).Encode(map[string]string{"deleted": id})
			} else {
				fmt.Printf("%s %s\n", c(red, "deleted:"), c(bold, id[:8]))
			}
			return
		}
		resp.Body.Close()
	}

	fmt.Fprintf(os.Stderr, "%s not found: %s\n", c(red, "error:"), id)
	os.Exit(1)
}

func cmdConfig(cfg Config, args []string) {
	if len(args) > 0 && args[0] == "set" {
		if len(args) < 3 {
			fmt.Fprintf(os.Stderr, "usage: share-home config set <key> <value>\n")
			os.Exit(1)
		}
		switch args[1] {
		case "url":
			cfg.URL = args[2]
		case "token":
			cfg.Token = args[2]
		default:
			fmt.Fprintf(os.Stderr, "%s unknown key: %s\n", c(red, "error:"), args[1])
			os.Exit(1)
		}
		if err := saveConfig(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "%s %s\n", c(red, "error saving:"), err)
			os.Exit(1)
		}
		fmt.Printf("%s\n", c(green, "config saved"))
		return
	}

	fmt.Printf("%s\n", c(bold, "config"))
	fmt.Printf("  url:   %s\n", c(cyan, orElse(cfg.URL, "(not set)")))
	fmt.Printf("  token: %s\n", orElse(maskToken(cfg.Token), "(not set)"))
	fmt.Printf("  file:  %s\n", c(dim, configPath()))
	if cfg.URL == "" {
		fmt.Printf("\n  %s\n", c(yellow, "first run: share-home config set url http://localhost:8080"))
	}
}

func cmdServer(cfg Config, args []string) {
	resp, err := do(cfg, "GET", "/", nil, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %s\n", c(red, "error:"), err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	t := time.Now()
	fmt.Printf("%s %s (%s)\n", c(green, "server:"), cfg.URL, c(dim, resp.Status))
	fmt.Printf("  time:  %s\n", t.Format(time.TimeOnly))
}

// Utilities

func copyToClipboard(text string) error {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	case "linux":
		// Try xclip first
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd := exec.Command("xclip", "-selection", "clipboard")
			cmd.Stdin = strings.NewReader(text)
			return cmd.Run()
		}
		// Try xsel
		if _, err := exec.LookPath("xsel"); err == nil {
			cmd := exec.Command("xsel", "--clipboard", "--input")
			cmd.Stdin = strings.NewReader(text)
			return cmd.Run()
		}
		return fmt.Errorf("no clipboard utility found (install xclip or xsel)")
	default:
		return fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}
}

func fmtSize(b int64) string {
	if b < 0 {
		return ""
	}
	units := []string{"B", "KB", "MB", "GB"}
	f := float64(b)
	for _, u := range units {
		if f < 1024 || u == units[len(units)-1] {
			if u == "B" {
				return fmt.Sprintf("%d%s", int(f), u)
			}
			return fmt.Sprintf("%.1f%s", f, u)
		}
		f /= 1024
	}
	return ""
}

func fileIcon(mime string) string {
	if strings.HasPrefix(mime, "image/") {
		return c(yellow, "🖼")
	}
	if strings.HasPrefix(mime, "video/") {
		return c(cyan, "🎬")
	}
	if strings.HasPrefix(mime, "text/") || mime == "application/pdf" {
		return c(green, "📄")
	}
	return c(dim, "📎")
}

func extractFilename(contentDisp, fallback string) string {
	if idx := strings.Index(contentDisp, "filename="); idx != -1 {
		fn := strings.Trim(contentDisp[idx+len("filename="):], `"'`)
		return fn
	}
	return fallback
}

func orElse(s, def string) string {
	if s == "" {
		return c(dim, def)
	}
	return c(cyan, s)
}

func maskToken(t string) string {
	if t == "" {
		return ""
	}
	if len(t) <= 4 {
		return "****"
	}
	return t[:4] + strings.Repeat("*", min(4, len(t)-4))
}

func findLink(cfg Config, id string) string {
	// try files
	resp, err := do(cfg, "GET", "/api/files", nil, "")
	if err == nil {
		var files []fileEntry
		json.NewDecoder(resp.Body).Decode(&files)
		resp.Body.Close()
		for _, f := range files {
			if f.ID == id || strings.HasPrefix(f.ID, id) {
				return cfg.URL + "/api/download/" + f.ID
			}
		}
	}

	// try clipboard
	resp, err = do(cfg, "GET", "/api/clipboard", nil, "")
	if err == nil {
		var clips []clipEntry
		json.NewDecoder(resp.Body).Decode(&clips)
		resp.Body.Close()
		for _, c2 := range clips {
			if c2.ID == id || strings.HasPrefix(c2.ID, id) {
				return cfg.URL + "/api/clipboard/" + c2.ID
			}
		}
	}

	// try urls
	resp, err = do(cfg, "GET", "/api/urls", nil, "")
	if err == nil {
		var urls []urlEntry
		json.NewDecoder(resp.Body).Decode(&urls)
		resp.Body.Close()
		for _, u := range urls {
			if u.Code == id || strings.HasPrefix(u.Code, id) {
				return cfg.URL + u.ShortURL
			}
		}
	}

	return ""
}

func openBrowser(u string) {
	if exec.Command("open", u).Run() != nil {
		if exec.Command("xdg-open", u).Run() != nil {
			fmt.Fprintf(os.Stderr, "%s couldn't open browser, copy: %s\n", c(red, "error:"), u)
		}
	}
}

func parseBoolFlag(args []string, flag string) (parsed []string, found bool) {
	for _, a := range args {
		if a == flag {
			found = true
		} else {
			parsed = append(parsed, a)
		}
	}
	return
}

// Main

func usage() {
	cmds := []string{"upload", "paste", "url", "list", "get", "open", "copy", "cat", "delete", "config", "server"}

	fmt.Fprintf(os.Stderr, "%sshare-home%s - share from your terminal\n\n", bold, reset)
	fmt.Fprintf(os.Stderr, "%sUsage:%s\n  %sshare-home%s <command> [args]\n\n", bold, reset, bold, reset)
	fmt.Fprintf(os.Stderr, "%sCommands:%s\n", bold, reset)
	for i, cmd := range cmds {
		switch i {
		case 0:
			fmt.Fprintf(os.Stderr, "  %s%s%s <file> %s[--expires 1h|1d|1w]%s   Upload a file\n", bold, cmd, reset, bold, reset)
		case 1:
			fmt.Fprintf(os.Stderr, "  %s%s%s <text>                                Save text to clipboard (or pipe via stdin)\n", bold, cmd, reset)
		case 2:
			fmt.Fprintf(os.Stderr, "  %s%s%s <url>                                   Shorten a URL\n", bold, cmd, reset)
		case 3:
			fmt.Fprintf(os.Stderr, "  %s%s%s [files|clip|urls]                      List all items\n", bold, cmd, reset)
		case 4:
			fmt.Fprintf(os.Stderr, "  %s%s%s <id>                                    Download file or view text\n", bold, cmd, reset)
		case 5:
			fmt.Fprintf(os.Stderr, "  %s%s%s <id>                                    Open in browser\n", bold, cmd, reset)
		case 6:
			fmt.Fprintf(os.Stderr, "  %s%s%s <id>                                    Copy link to clipboard\n", bold, cmd, reset)
		case 7:
			fmt.Fprintf(os.Stderr, "  %s%s%s <id>                                    Cat clipboard text to terminal\n", bold, cmd, reset)
		case 8:
			fmt.Fprintf(os.Stderr, "  %s%s%s <id>                                 Delete an item\n", bold, cmd, reset)
		case 9:
			fmt.Fprintf(os.Stderr, "  %s%s%s [set <key> <val>]                   Show or set config\n", bold, cmd, reset)
		case 10:
			fmt.Fprintf(os.Stderr, "  %s%s%s                                     Check server status\n", bold, cmd, reset)
		}
	}
	fmt.Fprintf(os.Stderr, "\n%sOptions:%s\n  --json    Output as JSON (for scripting)\n\n", bold, reset)
	fmt.Fprintf(os.Stderr, "%sExamples:%s\n", bold, reset)
	fmt.Fprintf(os.Stderr, "  %sshare-home upload photo.jpg%s\n", bold, reset)
	fmt.Fprintf(os.Stderr, "  %sshare-home paste \"hello world\"%s\n", bold, reset)
	fmt.Fprintf(os.Stderr, "  %sshare-home paste < long-text.txt%s\n", bold, reset)
	fmt.Fprintf(os.Stderr, "  %sshare-home url https://example.com%s\n", bold, reset)
	fmt.Fprintf(os.Stderr, "  %sshare-home list%s\n", bold, reset)
	fmt.Fprintf(os.Stderr, "  %sshare-home copy a1b2c3%s\n", bold, reset)
	fmt.Fprintf(os.Stderr, "  %sshare-home open a1b2c3%s\n\n", bold, reset)
	fmt.Fprintf(os.Stderr, "%sConfig:%s\n", bold, reset)
	fmt.Fprintf(os.Stderr, "  %sshare-home config set url http://localhost:8080%s\n", bold, reset)
	fmt.Fprintf(os.Stderr, "  %sshare-home config set token YOUR_TOKEN%s\n\n", bold, reset)
	fmt.Fprintf(os.Stderr, "  Or use env vars: %sSHARE_HOME_URL%s, %sSHARE_HOME_TOKEN%s\n", bold, reset, bold, reset)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cfg := loadConfig()
	args := os.Args[2:]

	// Parse global flags
	args, jsonOut := parseBoolFlag(args, "--json")

	cmd := os.Args[1]

	// Commands that don't need server URL
	switch cmd {
	case "config":
		cmdConfig(cfg, args)
		return
	case "--help", "-h", "help", "usage":
		usage()
		return
	}

	if cfg.URL == "" {
		fmt.Fprintf(os.Stderr, "%s SHARE_HOME_URL not set\n\nrun: %sshare-home config set url http://localhost:8080%s\n\n", c(red, "error:"), bold, reset)
		os.Exit(1)
	}

	switch cmd {
	case "upload":
		cmdUpload(cfg, args, jsonOut)
	case "paste", "p":
		cmdPaste(cfg, args, jsonOut, "")
	case "url", "shorten":
		cmdURL(cfg, args, jsonOut)
	case "list", "ls":
		cmdList(cfg, args, jsonOut)
	case "get":
		cmdGet(cfg, args, jsonOut)
	case "open":
		cmdOpen(cfg, args)
	case "copy", "cp":
		cmdCopy(cfg, args, jsonOut)
	case "cat":
		cmdCat(cfg, args)
	case "delete", "del", "rm", "remove":
		cmdDelete(cfg, args, jsonOut)
	case "server", "ping":
		cmdServer(cfg, args)
	default:
		fmt.Fprintf(os.Stderr, "%s unknown command: %s\n\n", c(red, "error:"), cmd)
		usage()
	}
}

// silence unused import warnings
var _ = strconv.Itoa
var _ = time.Now
var _ = sort.Sort
