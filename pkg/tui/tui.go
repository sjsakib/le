package tui

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mdp/qrterminal/v3"
	"go.sakib.dev/le/pkg/utils"

	"go.sakib.dev/le/pkg/server"
	"go.sakib.dev/le/pkg/state"
)

type model struct {
	srvr  *server.Server
	state *state.ServerState
}

func newModel(srvr *server.Server) model {
	return model{
		srvr:  srvr,
		state: state.New(),
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func clamp[T cmp.Ordered](v T, mn, mx T) T {
	return max(mn, min(v, mx))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			m.srvr.Stop()
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
	m.state.RLock()
	defer m.state.RUnlock()

	state := m.state
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

	var str strings.Builder
	fmt.Fprintf(&str, "Server running at: %s\n", *state.Addr)

	fmt.Fprintf(&str, "From directory %s\n", state.Dir)

	str.WriteString(stringWriter.String())

	str.WriteString("\n")

	downloadKeys := make([]*string, 0, len(state.Downloads))

	for i := range state.Downloads {
		downloadKeys = append(downloadKeys, &i)
	}

	// sort keys with StartedAt
	sort.Slice(downloadKeys, func(i, j int) bool {
		return state.Downloads[*downloadKeys[i]].StartedAt.Before(state.Downloads[*downloadKeys[j]].StartedAt)
	})

	for _, d := range downloadKeys {
		d := state.Downloads[*d]
		fmt.Fprintf(&str, "%s\n ", d.FileDisplayPath)

		if d.Chunks == nil {
			slog.Warn("No chunks found for download", "download_id", d.ID)
		}

		barBlockCount := 50
		blockLevels := []string{" ", "▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"}
		blcokLevelCount := len(blockLevels)

		totalRequested := int64(0)
		totalSent := int64(0)

		blocksRendered := 0
		start := int64(0)
		end := int64(0)

		blockStr := strings.Builder{}
		for i, c := range d.Chunks {
			if d.TotalSize < 0 {
				totalSent += c.Sent
				totalRequested = -1
				break
			}
			if i == 0 {
				start = c.Start
			}
			end = c.End
			if i != len(d.Chunks)-1 {
				nextChunk := d.Chunks[i+1]
				if nextChunk.Start < c.End {
					end = nextChunk.Start
				}
			}

			chunkSize := max(end-start+1, 0)
			totalRequested += chunkSize

			chunkProportion := float64(chunkSize) / float64(d.TotalSize)
			chunkBlockCount := int(float64(barBlockCount) * chunkProportion)

			blocksRendered += chunkBlockCount

			if i == len(d.Chunks)-1 {
				chunkBlockCount += barBlockCount - blocksRendered
			}

			chunkProgress := float64(c.Sent) / float64(chunkSize)
			chunkProgressBlocks := chunkProgress * float64(chunkBlockCount)

			for j := 0; j < int(chunkBlockCount); j++ {
				blockIdx := int(clamp((chunkProgressBlocks-float64(j))*(float64(blcokLevelCount)), 0, float64(blcokLevelCount)-1))
				blockStr.WriteString(blockLevels[blockIdx])
			}
			start = end

			if c.Sent >= chunkSize {
				totalSent += chunkSize
			} else {
				totalSent += c.Sent
			}
		}

		totalProgress := float64(totalSent) / float64(totalRequested)

		if totalRequested < 0 {
			fmt.Fprintf(&str, "%s / ?\n\n",
				utils.HumanizeSize(totalSent),
			)
		} else {
			fmt.Fprintf(&str, "%5.2f%% %s  %s / %s\n\n",
				totalProgress*100,
				blockStr.String(),
				utils.HumanizeSize(totalSent),
				utils.HumanizeSize(totalRequested))
		}
	}

	str.WriteString("\nPress Ctrl+C or 'q' to quit.\n\n")

	return str.String()
}

func Start(ctx context.Context, srvr *server.Server) error {
	model := newModel(srvr)
	p := tea.NewProgram(model, tea.WithAltScreen())

	ch := make(chan server.ServerEvent, 100)
	srvr.Subscribe(ch)

	go func() {
		for event := range ch {
			model.state.HandleEvent(event)
			p.Send(tea.Msg("update"))
		}
	}()

	// Save original stdout
	old := os.Stdout

	defer func() {
		os.Stdout = old // Restore original stdout
		p.Send(tea.Quit())
		close(ch)
	}()

	go func() {
		<-ctx.Done()
		p.Send(tea.Quit())
	}()

	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}
