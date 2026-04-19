package llm

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

type InteractivePrompter struct {
	In  io.Reader
	Out io.Writer
}

func NewInteractivePrompter(in io.Reader, out io.Writer) *InteractivePrompter {
	return &InteractivePrompter{In: in, Out: out}
}

func (p *InteractivePrompter) ChooseSetupOption(_ context.Context, prompt SetupPrompt) (SetupChoice, error) {
	reader := bufio.NewReader(p.In)
	if _, err := fmt.Fprintln(p.Out, "Explain mode setup:"); err != nil {
		return "", err
	}
	if _, err := fmt.Fprintf(p.Out, "1) Download %s as llamafile\n", prompt.LlamafileName); err != nil {
		return "", err
	}
	if prompt.HasOllama {
		if _, err := fmt.Fprintln(p.Out, "2) Use existing Ollama (localhost:11434)"); err != nil {
			return "", err
		}
	} else {
		if _, err := fmt.Fprintln(p.Out, "2) Use Ollama (not currently reachable)"); err != nil {
			return "", err
		}
	}
	if _, err := fmt.Fprintln(p.Out, "3) Skip explain setup and do not ask again"); err != nil {
		return "", err
	}
	if _, err := fmt.Fprint(p.Out, "Choose [1/2/3]: "); err != nil {
		return "", err
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	switch strings.TrimSpace(line) {
	case "1":
		return SetupChoiceLlamafile, nil
	case "2":
		return SetupChoiceOllama, nil
	case "3":
		return SetupChoiceSkip, nil
	default:
		return SetupChoiceSkip, nil
	}
}

func (p *InteractivePrompter) ConfirmDownload(_ context.Context, url, fileName string) (bool, error) {
	reader := bufio.NewReader(p.In)
	if _, err := fmt.Fprintf(p.Out, "Download %s from %s? [y/N]: ", fileName, url); err != nil {
		return false, err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func (p *InteractivePrompter) UpdateDownloadProgress(downloadedBytes, totalBytes int64) {
	if totalBytes > 0 {
		percent := float64(downloadedBytes) / float64(totalBytes) * 100
		_, _ = fmt.Fprintf(p.Out, "\rDownloading llamafile... %6.2f%%", percent)
		if downloadedBytes >= totalBytes {
			_, _ = fmt.Fprintln(p.Out)
		}
		return
	}
	_, _ = fmt.Fprintf(p.Out, "\rDownloading llamafile... %d bytes", downloadedBytes)
}
