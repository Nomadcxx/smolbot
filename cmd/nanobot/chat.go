package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	glamour "charm.land/glamour/v2"
	"github.com/spf13/cobra"

	"github.com/Nomadcxx/nanobot-go/pkg/config"
)

type chatIO struct {
	In  io.Reader
	Out io.Writer
}

var newReadlineSession = func(in *chatIO, out io.Writer) (readlineSession, error) {
	return newBubbleteaReadline(in, out)
}

type chatSessionDeps struct {
	HistoryPath string
	Signals     <-chan os.Signal
}

type chatRenderer interface {
	Render(string) (string, error)
}

var runInteractiveChat = runInteractiveChatImpl

var newTermRenderer = func() (chatRenderer, error) {
	return glamour.NewTermRenderer()
}

var chatSpinnerInterval = 150 * time.Millisecond

var errChatExitRequested = errors.New("chat exit requested")

func newChatCmd(opts *rootOptions) *cobra.Command {
	var message string
	var session string
	var markdown bool

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Run an in-process chat session",
		RunE: func(cmd *cobra.Command, args []string) error {
			req := chatRequest{
				Session:    session,
				Message:    message,
				Markdown:   markdown,
				ConfigPath: defaultConfigPath(*opts),
				Workspace:  opts.workspace,
			}
			if strings.TrimSpace(req.Message) == "" {
				return runInteractiveChat(context.Background(), req, &chatIO{
					In:  cmd.InOrStdin(),
					Out: cmd.OutOrStdout(),
				})
			}
			output, err := runChatMessage(context.Background(), req)
			if err != nil {
				return err
			}
			if output != "" {
				fmt.Fprintln(cmd.OutOrStdout(), renderChatOutput(output, markdown))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", "Send one message and exit")
	cmd.Flags().StringVar(&session, "session", "", "Reuse an existing session")
	cmd.Flags().BoolVar(&markdown, "markdown", false, "Print markdown-friendly output")
	return cmd
}

func runInteractiveChatImpl(ctx context.Context, req chatRequest, c *chatIO) error {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signalCh)

	return runInteractiveChatSession(ctx, req, c, chatSessionDeps{
		HistoryPath: config.DefaultPaths().ChatHistory(),
		Signals:     signalCh,
	})
}

func runInteractiveChatSession(ctx context.Context, req chatRequest, chatIO *chatIO, deps chatSessionDeps) error {
	if chatIO == nil || chatIO.In == nil || chatIO.Out == nil {
		return fmt.Errorf("interactive chat requires stdin/stdout")
	}

	rl, err := newReadlineSession(chatIO, chatIO.Out)
	if err != nil {
		return fmt.Errorf("failed to create readline session: %w", err)
	}
	defer rl.Close()

	signalCh := deps.Signals
	if deps.HistoryPath == "" {
		deps.HistoryPath = config.DefaultPaths().ChatHistory()
	}

	for {
		line, err := rl.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) || err.Error() == "EOF" {
				return nil
			}
			return err
		}

		line = strings.TrimSpace(sanitizeInteractiveLine(line))
		if line == "" {
			continue
		}
		if line == "/quit" || line == "/exit" {
			return nil
		}

		rl.AddToHistory(line)
		if err := appendChatHistory(deps.HistoryPath, line); err != nil {
			return err
		}

		output, err := runChatTurn(ctx, req, line, signalCh, chatIO.Out)
		if err != nil {
			if errors.Is(err, errChatExitRequested) {
				return nil
			}
			if errors.Is(err, context.Canceled) {
				continue
			}
			if _, writeErr := fmt.Fprintf(chatIO.Out, "error: %v\n", err); writeErr != nil {
				return writeErr
			}
			continue
		}
		if output == "" {
			continue
		}
		if _, err := fmt.Fprintln(chatIO.Out, renderChatOutput(output, req.Markdown)); err != nil {
			return err
		}
	}
}

func nextChatInput(ctx context.Context, inputCh <-chan string, errCh <-chan error, signalCh <-chan os.Signal) (string, bool, error) {
	for {
		select {
		case <-ctx.Done():
			return "", false, ctx.Err()
		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if err != nil {
				return "", false, err
			}
			errCh = nil
		case line, ok := <-inputCh:
			if !ok {
				return "", false, nil
			}
			return line, true, nil
		case sig, ok := <-signalCh:
			if !ok {
				signalCh = nil
				continue
			}
			if sig == os.Interrupt {
				return "", false, context.Canceled
			}
			if sig == syscall.SIGTERM || sig == syscall.SIGHUP {
				return "", false, errChatExitRequested
			}
		}
	}
}

func runChatTurn(ctx context.Context, req chatRequest, line string, signalCh <-chan os.Signal, out io.Writer) (string, error) {
	turnCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	resultCh := make(chan struct {
		output string
		err    error
	}, 1)
	go func() {
		output, err := runChatMessage(turnCtx, chatRequest{
			Session:    req.Session,
			Message:    line,
			Markdown:   req.Markdown,
			ConfigPath: req.ConfigPath,
			Workspace:  req.Workspace,
		})
		resultCh <- struct {
			output string
			err    error
		}{output: output, err: err}
	}()

	var spinnerC <-chan time.Time
	if out != nil && chatSpinnerInterval > 0 {
		ticker := time.NewTicker(chatSpinnerInterval)
		defer ticker.Stop()
		spinnerC = ticker.C
	}

	spinnerShown := false
	interrupt := false
	exitRequested := false
	for {
		select {
		case result := <-resultCh:
			if spinnerShown && out != nil {
				if _, err := fmt.Fprintln(out); err != nil {
					return "", err
				}
			}
			if exitRequested {
				return "", errChatExitRequested
			}
			if interrupt && errors.Is(result.err, context.Canceled) {
				return "", context.Canceled
			}
			return result.output, result.err
		case <-spinnerC:
			if !spinnerShown && out != nil {
				if _, err := fmt.Fprint(out, "Thinking...\n"); err != nil {
					return "", err
				}
				spinnerShown = true
			}
		case sig, ok := <-signalCh:
			if !ok {
				signalCh = nil
				continue
			}
			switch sig {
			case os.Interrupt:
				interrupt = true
				cancel()
			case syscall.SIGTERM, syscall.SIGHUP:
				exitRequested = true
				cancel()
			}
		case <-ctx.Done():
			cancel()
			return "", ctx.Err()
		}
	}
}

func sanitizeInteractiveLine(line string) string {
	line = strings.ReplaceAll(line, "\x1b[200~", "")
	line = strings.ReplaceAll(line, "\x1b[201~", "")
	return line
}

func appendChatHistory(historyPath, line string) error {
	if historyPath == "" {
		historyPath = config.DefaultPaths().ChatHistory()
	}
	if err := os.MkdirAll(filepath.Dir(historyPath), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(historyPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := fmt.Fprintln(file, line); err != nil {
		return err
	}
	return nil
}

func renderChatOutput(output string, markdown bool) string {
	if !markdown {
		return output
	}
	renderer, err := newTermRenderer()
	if err != nil {
		return output
	}
	rendered, err := renderer.Render(output)
	if err != nil {
		return output
	}
	return strings.TrimRight(rendered, "\n")
}
