package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultOllamaURL       = "http://localhost:11434"
	defaultOllamaModel     = "qwen2.5:1.5b"
	defaultLlamafileURL    = "https://huggingface.co/Mozilla/Qwen2.5-1.5B-Instruct-llamafile/resolve/main/Qwen2.5-1.5B-Instruct.Q6_K.llamafile"
	defaultLlamafileName   = "qwen2.5-1.5b.llamafile"
	defaultResponseWordCap = 150
)

type ExplainRequest struct {
	Command      string
	Question     string
	MoreDetail   bool
	MaxWords     int
	OriginalText string
}

type TokenHandler func(token string) error

type Explainer interface {
	ProviderName() string
	Explain(ctx context.Context, req ExplainRequest, onToken TokenHandler) error
}

type AutoOptions struct {
	ConfigDir      string
	HTTPClient     *http.Client
	Prompter       SetupPrompter
	OllamaBaseURL  string
	OllamaModel    string
	LlamafileURL   string
	LlamafileName  string
	LlamafileFlags []string
}

type SetupChoice string

const (
	SetupChoiceLlamafile SetupChoice = "llamafile"
	SetupChoiceOllama    SetupChoice = "ollama"
	SetupChoiceSkip      SetupChoice = "skip"
)

type SetupPrompt struct {
	HasOllama     bool
	LlamafileURL  string
	LlamafileName string
}

type SetupPrompter interface {
	ChooseSetupOption(ctx context.Context, prompt SetupPrompt) (SetupChoice, error)
	ConfirmDownload(ctx context.Context, url, fileName string) (bool, error)
	UpdateDownloadProgress(downloadedBytes, totalBytes int64)
}

type LlamafileExplainer struct {
	ExecutablePath string
	Flags          []string
}

type OllamaExplainer struct {
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

type FallbackExplainer struct {
	options AutoOptions
	config  runtimeConfig
}

type runtimeConfig struct {
	SkipLLM bool   `json:"skip_llm"`
	Engine  string `json:"engine"`
}

func NewExplainer(ctx context.Context, opts AutoOptions) (Explainer, error) {
	opts = withDefaults(opts)
	conf, _ := loadRuntimeConfig(opts.ConfigDir)

	if conf.SkipLLM {
		return &FallbackExplainer{options: opts, config: conf}, nil
	}

	if llamafilePath, ok := findLlamafile(opts.ConfigDir); ok {
		return &LlamafileExplainer{ExecutablePath: llamafilePath, Flags: opts.LlamafileFlags}, nil
	}

	if isOllamaReachable(ctx, opts.HTTPClient, opts.OllamaBaseURL) {
		return &OllamaExplainer{BaseURL: opts.OllamaBaseURL, Model: opts.OllamaModel, HTTPClient: opts.HTTPClient}, nil
	}

	return &FallbackExplainer{options: opts, config: conf}, nil
}

func (l *LlamafileExplainer) ProviderName() string { return "llamafile" }

func (l *LlamafileExplainer) Explain(ctx context.Context, req ExplainRequest, onToken TokenHandler) error {
	if strings.TrimSpace(l.ExecutablePath) == "" {
		return fmt.Errorf("llamafile executable path is empty")
	}

	args := append([]string{}, l.Flags...)
	if len(args) == 0 {
		args = []string{"-p", composePrompt(req)}
	} else {
		args = append(args, composePrompt(req))
	}

	cmd := exec.CommandContext(ctx, l.ExecutablePath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open llamafile stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("open llamafile stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start llamafile: %w", err)
	}

	streamErr := make(chan error, 2)
	go func() { streamErr <- streamWords(stdout, onToken) }()
	go func() { streamErr <- drain(stderr) }()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("llamafile failed: %w", err)
	}

	for i := 0; i < 2; i++ {
		if err := <-streamErr; err != nil {
			return err
		}
	}

	return nil
}

func (o *OllamaExplainer) ProviderName() string { return "ollama" }

func (o *OllamaExplainer) Explain(ctx context.Context, req ExplainRequest, onToken TokenHandler) error {
	client := o.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 0}
	}

	body := map[string]any{
		"model":  orDefault(o.Model, defaultOllamaModel),
		"prompt": buildUserPrompt(req),
		"system": strictSystemPrompt(req),
		"stream": true,
	}
	payload, _ := json.Marshal(body)

	endpoint := strings.TrimRight(orDefault(o.BaseURL, defaultOllamaURL), "/") + "/api/generate"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build ollama request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("call ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	type chunk struct {
		Response string `json:"response"`
		Done     bool   `json:"done"`
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var c chunk
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return fmt.Errorf("decode ollama stream: %w", err)
		}
		if c.Response != "" && onToken != nil {
			if err := onToken(c.Response); err != nil {
				return err
			}
		}
		if c.Done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read ollama stream: %w", err)
	}

	return nil
}

func (f *FallbackExplainer) ProviderName() string { return "fallback" }

func (f *FallbackExplainer) Explain(ctx context.Context, req ExplainRequest, onToken TokenHandler) error {
	if f.config.SkipLLM {
		if onToken != nil {
			_ = onToken("Explain mode is disabled by your saved preference.\n")
		}
		return nil
	}

	if f.options.Prompter == nil {
		return fmt.Errorf("no LLM provider is available and no setup prompter is configured")
	}

	prompt := SetupPrompt{
		HasOllama:     isOllamaReachable(ctx, f.options.HTTPClient, f.options.OllamaBaseURL),
		LlamafileURL:  f.options.LlamafileURL,
		LlamafileName: f.options.LlamafileName,
	}
	choice, err := f.options.Prompter.ChooseSetupOption(ctx, prompt)
	if err != nil {
		return err
	}

	switch choice {
	case SetupChoiceSkip:
		f.config.SkipLLM = true
		if err := saveRuntimeConfig(f.options.ConfigDir, f.config); err != nil {
			return err
		}
		if onToken != nil {
			_ = onToken("Saved preference: skip explain setup in future sessions.\n")
		}
		return nil

	case SetupChoiceOllama:
		if !prompt.HasOllama {
			return fmt.Errorf("ollama is not reachable on %s", f.options.OllamaBaseURL)
		}
		explainer := &OllamaExplainer{BaseURL: f.options.OllamaBaseURL, Model: f.options.OllamaModel, HTTPClient: f.options.HTTPClient}
		return explainer.Explain(ctx, req, onToken)

	case SetupChoiceLlamafile:
		confirm, err := f.options.Prompter.ConfirmDownload(ctx, f.options.LlamafileURL, f.options.LlamafileName)
		if err != nil {
			return err
		}
		if !confirm {
			return fmt.Errorf("llamafile setup canceled by user")
		}

		path, err := downloadLlamafile(ctx, f.options, f.options.Prompter)
		if err != nil {
			return err
		}
		explainer := &LlamafileExplainer{ExecutablePath: path, Flags: f.options.LlamafileFlags}
		return explainer.Explain(ctx, req, onToken)
	}

	return fmt.Errorf("unknown setup choice: %s", choice)
}

func withDefaults(opts AutoOptions) AutoOptions {
	if opts.ConfigDir == "" {
		home, _ := os.UserHomeDir()
		opts.ConfigDir = filepath.Join(home, ".config", "shellai")
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if opts.Prompter == nil {
		opts.Prompter = NewInteractivePrompter(os.Stdin, os.Stdout)
	}
	opts.OllamaBaseURL = orDefault(opts.OllamaBaseURL, defaultOllamaURL)
	opts.OllamaModel = orDefault(opts.OllamaModel, defaultOllamaModel)
	opts.LlamafileURL = orDefault(opts.LlamafileURL, defaultLlamafileURL)
	opts.LlamafileName = orDefault(opts.LlamafileName, defaultLlamafileName)
	return opts
}

func findLlamafile(configDir string) (string, bool) {
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".llamafile") {
			path := filepath.Join(configDir, e.Name())
			return path, true
		}
	}
	return "", false
}

func isOllamaReachable(ctx context.Context, client *http.Client, baseURL string) bool {
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}

	endpoint := strings.TrimRight(orDefault(baseURL, defaultOllamaURL), "/") + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) {
			return false
		}
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func composePrompt(req ExplainRequest) string {
	return strictSystemPrompt(req) + "\n\n" + buildUserPrompt(req)
}

func strictSystemPrompt(req ExplainRequest) string {
	maxWords := req.MaxWords
	if maxWords <= 0 {
		maxWords = defaultResponseWordCap
	}
	if req.MoreDetail && maxWords < 300 {
		maxWords = 300
	}

	return fmt.Sprintf("You explain shell commands. Rules: explain clearly and concisely; never suggest running commands; never go off topic; use plain technical English; limit response to at most %d words unless user explicitly asked for more detail.", maxWords)
}

func buildUserPrompt(req ExplainRequest) string {
	if strings.TrimSpace(req.Question) != "" {
		return strings.TrimSpace(req.Question)
	}
	return fmt.Sprintf("Explain this shell command: %s", strings.TrimSpace(req.Command))
}

func loadRuntimeConfig(configDir string) (runtimeConfig, error) {
	path := filepath.Join(configDir, "preferences.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return runtimeConfig{}, err
	}
	var conf runtimeConfig
	if err := json.Unmarshal(data, &conf); err != nil {
		return runtimeConfig{}, err
	}
	return conf, nil
}

func saveRuntimeConfig(configDir string, conf runtimeConfig) error {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(configDir, "preferences.json")
	data, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func downloadLlamafile(ctx context.Context, opts AutoOptions, progress SetupPrompter) (string, error) {
	if err := os.MkdirAll(opts.ConfigDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(opts.ConfigDir, opts.LlamafileName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, opts.LlamafileURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer out.Close()

	total := resp.ContentLength
	buf := make([]byte, 32*1024)
	var downloaded int64
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				return "", err
			}
			downloaded += int64(n)
			if progress != nil {
				progress.UpdateDownloadProgress(downloaded, total)
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}

	if err := verifyLlamafile(path); err != nil {
		return "", err
	}
	if err := os.Chmod(path, 0o755); err != nil {
		return "", err
	}

	return path, nil
}

func verifyLlamafile(path string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}
	if stat.Size() < 10*1024*1024 {
		return fmt.Errorf("downloaded llamafile is unexpectedly small")
	}
	return nil
}

func streamWords(r io.Reader, onToken TokenHandler) error {
	if onToken == nil {
		_, err := io.Copy(io.Discard, r)
		return err
	}
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanWords)
	for scanner.Scan() {
		if err := onToken(scanner.Text() + " "); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func drain(r io.Reader) error {
	_, err := io.Copy(io.Discard, r)
	return err
}

func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
