// Package screpsim runs the OpenBW engine (github.com/marianogappa/openbw-go) on a replay that screp
// (github.com/icza/screp) parses, and joins the engine's facts to screp's own values: commands by their index
// in Commands.Cmds, players by slot, and units by screp's unit tags. It returns screp's types unchanged and
// reports raw facts only.
package screpsim

import (
	"fmt"
	"io"
	"iter"
	"log"

	"github.com/icza/screp/rep"
	"github.com/icza/screp/rep/repcmd"
	"github.com/icza/screp/repparser"
	openbw "github.com/marianogappa/openbw-go"
)

// Options configures Open.
type Options struct {
	// Engine configures openbw.Open. Subscribe openbw.KindCommandResult to get CommandResult events.
	Engine openbw.Options
	// Parse configures screp's parser. Commands are always parsed. A nil Logger discards screp's log output.
	Parse repparser.Config
}

// Game is one replay parsed by screp and played by the engine. It is not safe for concurrent use.
type Game struct {
	replay  *rep.Replay
	engine  *openbw.Game
	links   []CommandLink
	actions []int
	scrTags bool
}

// Open parses replay with screp and opens it in the engine at frame 0.
// The caller must not change replay while the game is in use.
func Open(replay []byte, opts Options) (*Game, error) {
	cfg := opts.Parse
	cfg.Commands = true
	if cfg.Logger == nil {
		cfg.Logger = log.New(io.Discard, "", 0)
	}
	parsed, err := repparser.ParseConfig(replay, cfg)
	if err != nil {
		return nil, fmt.Errorf("screpsim: parse replay: %w", err)
	}
	engine, err := openbw.Open(replay, opts.Engine)
	if err != nil {
		return nil, fmt.Errorf("screpsim: %w", err)
	}
	g := &Game{replay: parsed, engine: engine, scrTags: engine.Info().SourceFormat == format121}
	g.links, g.actions = linkCommands(parsed.Commands, engine.SourceCommands())
	return g, nil
}

// Replay is screp's parse of the replay.
func (g *Game) Replay() *rep.Replay { return g.replay }

// Engine is the engine's game, for state reads, AdvanceTo, Done and the other openbw.Game methods.
func (g *Game) Engine() *openbw.Game { return g.engine }

// Err is the engine error that stopped the game, or nil. Check it after the loop.
func (g *Game) Err() error { return g.engine.Err() }

// Events yields the engine's events, as openbw.Game.Events does. Each *openbw.CommandResult comes as a
// *CommandResult that adds its screp command.
func (g *Game) Events() iter.Seq[openbw.Event] {
	return func(yield func(openbw.Event) bool) {
		for e := range g.engine.Events() {
			if r, ok := e.(*openbw.CommandResult); ok {
				e = g.join(r)
			}
			if !yield(e) {
				return
			}
		}
	}
}

// CommandResult is the engine's result of one command, with the screp command it executes.
type CommandResult struct {
	*openbw.CommandResult
	// CmdIndex is the index of Cmd in Replay().Commands.Cmds, or -1 if the link is not exact.
	CmdIndex int
	// Cmd is screp's command, or nil if CmdIndex is -1.
	Cmd repcmd.Cmd
}

func (g *Game) join(r *openbw.CommandResult) *CommandResult {
	j := &CommandResult{CommandResult: r, CmdIndex: -1}
	if r.Action >= 0 && int(r.Action) < len(g.actions) {
		j.CmdIndex = g.actions[r.Action]
	}
	if j.CmdIndex >= 0 {
		j.Cmd = g.replay.Commands.Cmds[j.CmdIndex]
	}
	return j
}

// CommandLink is the engine's side of one screp command.
type CommandLink struct {
	// Source is the index in Engine().SourceCommands(), or -1 if the link is not exact.
	Source int32
	// Action is the engine action that executes the command, or -1 if the engine does not execute it
	// (the downgrade dropped it, see SourceCommands()[Source].Dropped) or the link is not exact.
	Action int32
}

// CommandLinks has one CommandLink per screp command, in Replay().Commands.Cmds order. Each call returns a new slice.
func (g *Game) CommandLinks() []CommandLink { return append([]CommandLink(nil), g.links...) }

// Player is screp's player in engine slot owner (Player.SlotID == owner), or nil.
func (g *Game) Player(owner int8) *rep.Player {
	if g.replay.Header == nil || owner < 0 {
		return nil
	}
	for _, p := range g.replay.Header.Players {
		if p.SlotID == uint16(owner) {
			return p
		}
	}
	return nil
}
