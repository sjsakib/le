package tui

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mdp/qrterminal/v3"
	"go.sakib.dev/le/server"
)

type model struct {
	srvr *server.Server
}

func newModel(srvr *server.Server) model {
	return model{
		srvr: srvr,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
	case string:
		if msg == "update" {
			// Handle update messages, e.g., refresh the view
			return m, nil
		}
	default:
		// Handle other messages if necessary
	}

	return m, nil
}

func (m model) View() string {
	state := m.srvr.GetState()
	if state.Addr == nil {
		// return a loading indicator
		return "Loading server address...\nPress Ctrl+C or 'q' to quit.\n"
	}

	stringWriter := &strings.Builder{}

	qrterminal.GenerateWithConfig(*state.Addr, qrterminal.Config{
		Level:      qrterminal.L,
		Writer:     stringWriter,
		HalfBlocks: true,
		BlackChar:  qrterminal.BLACK_BLACK,
	})

	connCount := len(state.Conns)
	str := fmt.Sprintf("Server running at: %s\nNumber of connections: %d\n", *state.Addr, connCount)

	str += fmt.Sprintf("From directory %s\n", state.Dir)

	str += stringWriter.String()

	str += "\n"

	for _, d := range state.Downloads {
		str += fmt.Sprintf("%s\n", d.FileDisplayPath)

		if d.Chunks == nil {
			slog.Warn("No chunks found for download", "download_id", d.ID)
		}

		for _, c := range d.Chunks {
			slog.Debug("Chunk info", "chunk", c)
			slog.Debug("Sent from tui", "sent", c.Sent)
			str += fmt.Sprintf("  %s - %d/%d\n", c.ConnID, c.Sent, c.End-c.Start+1)
		}
		str += "\n"
	}

	str += "\nPress Ctrl+C or 'q' to quit.\n\n"

	return str
}

func Start(srvr *server.Server, ch <-chan server.ServerEventName) error {
	p := tea.NewProgram(newModel(srvr), tea.WithAltScreen())

	go func() {
		for range ch {
			p.Send(tea.Msg("update"))
		}
	}()

	// Save original stdout
	old := os.Stdout

	// Redirect stdout to /dev/null
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull

	if _, err := p.Run(); err != nil {
		return err
	}

	os.Stdout = old // Restore original stdout
	return nil
}
