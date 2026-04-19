package llm

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/glamour"
)

type StreamRenderer struct {
	out      io.Writer
	renderer *glamour.TermRenderer
	buffer   strings.Builder
}

func NewStreamRenderer(out io.Writer, width int) (*StreamRenderer, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil, err
	}

	return &StreamRenderer{
		out:      out,
		renderer: renderer,
	}, nil
}

func (r *StreamRenderer) WriteToken(token string) error {
	r.buffer.WriteString(token)
	rendered, err := r.renderer.Render(r.buffer.String())
	if err != nil {
		return err
	}

	// Clear and redraw for incremental markdown rendering in terminal UIs.
	if _, err := fmt.Fprint(r.out, "\x1b[H\x1b[2J"); err != nil {
		return err
	}
	_, err = fmt.Fprint(r.out, rendered)
	return err
}

func StreamExplainWithGlamour(ctx context.Context, explainer Explainer, req ExplainRequest, out io.Writer, width int) error {
	renderer, err := NewStreamRenderer(out, width)
	if err != nil {
		return err
	}
	return explainer.Explain(ctx, req, renderer.WriteToken)
}
