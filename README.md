# screpsim

Runs the StarCraft: Brood War engine ([openbw-go](https://github.com/marianogappa/openbw-go)) on a replay that [screp](https://github.com/icza/screp) parses, and joins what the engine knows to screp's own values.

```
screp  <-  screpsim  ->  openbw-go
```

- screp says what was clicked: "player 1 issued Train Zergling at frame 3000".
- The engine says what happened: the command was rejected for lack of minerals, or it made units 812 and 813, which died at frame 5200 to a Reaver.

screpsim returns screp's types unchanged (`*rep.Replay`, `repcmd.Cmd`, `*rep.Player`, `repcmd.UnitTag`) and adds engine facts keyed by screp's identifiers: the command index in `Commands.Cmds`, the player slot and the unit tag. It never copies or redefines a screp type.

It is a library. Output is Go values through a stream; it writes no files and does no analysis.

## Usage

```go
import (
	openbw "github.com/marianogappa/openbw-go"
	"github.com/marianogappa/screpsim"
)

g, err := screpsim.Open(replay, screpsim.Options{
	Engine: openbw.Options{Kinds: openbw.KindCommandResult | openbw.KindUnitDestroyed},
})
if err != nil {
	return err
}
for e := range g.Events() {
	switch e := e.(type) {
	case *screpsim.CommandResult:
		if e.Cmd != nil && e.Outcome == openbw.Rejected {
			fmt.Println(e.CmdIndex, g.Player(e.Owner).Name, e.Cmd.BaseCmd().Type.Name, e.Rejections)
		}
	case *openbw.UnitDestroyed:
		fmt.Println(e.Frame, e.Type, g.UnitTag(e.Unit))
	}
}
return g.Err()
```

## API

```go
func Open(replay []byte, opts Options) (*Game, error)

type Options struct {
	Engine openbw.Options   // the engine's options; subscribe openbw.KindCommandResult for joins
	Parse  repparser.Config // screp's options; Commands is always true; a nil Logger discards screp's log
}

func (g *Game) Replay() *rep.Replay                        // screp's parse
func (g *Game) Engine() *openbw.Game                       // state reads, AdvanceTo, Done, Header, SourceCommands, ...
func (g *Game) Events() iter.Seq[openbw.Event]             // the engine's stream, with *CommandResult for command results
func (g *Game) Err() error
func (g *Game) CommandLinks() []CommandLink                // one per screp command: {Source, Action}
func (g *Game) Player(owner int8) *rep.Player              // screp's player in an engine slot
func (g *Game) UnitTag(id openbw.UnitID) repcmd.UnitTag    // screp's tag for an engine unit

type CommandResult struct {
	*openbw.CommandResult
	CmdIndex int        // index in Replay().Commands.Cmds; -1 if the link is not exact
	Cmd      repcmd.Cmd // nil if CmdIndex is -1
}
```

- `Events` yields every engine event as `openbw.Game.Events` does. A command result comes as `*screpsim.CommandResult`, never as `*openbw.CommandResult`. Its `openbw.CommandResult` fields are promoted.
- The engine's `Owner` is the player slot: screp's `Player.SlotID`, not `Player.ID`.

### Command links

The engine runs a downgraded copy of the replay. `Engine().SourceCommands()` is the original replay's command stream as the downgrade read it, and each `openbw.CommandResult.Source` points into it. screpsim walks screp's commands and the source commands frame by frame, in step:

- A link needs the same frame, position in the frame, player ID and command ID (screp's virtual Land matches Build).
- In a frame, linking stops at the first mismatch, at a command whose length the downgrade could not read, and at screp's first parse error.
- A link is exact or absent (`CmdIndex` -1, `CommandLink.Source` -1). screpsim never guesses.

`CommandLinks()[i].Action` is -1 for a screp command that the engine does not execute: the downgrade dropped it (`Engine().SourceCommands()[Source].Dropped` says why) or it has no exact link.

### Unit tags

`UnitTag` gives the tag that screp reads from the replay's commands for an engine unit. For a legacy replay it is `UnitID.LegacyTag`. For a 1.21 replay, screp keeps the low 16 bits of the 32-bit tag (the 13-bit index and the generation modulo 8), and the index mapping is the downgrade's: index 1 stays 1, the others add 1700.

## Validation

`tools/linkcheck` plays replays and checks, for each one:

- Every command result with a link is in screp's order, has the frame, player and command type of its screp command, and agrees with `CommandLinks`.
- Every screp command that the engine executes has exactly one result. The others are dropped by the downgrade or after the end frame.
- For each Select whose units the engine all selected, `UnitTag` of each selected unit is one of screp's tags.

```
go run ./tools/linkcheck -list tools/linkcheck/replays.txt
```

On the 210 replays of `tools/linkcheck/replays.txt` (185 SC:R, 25 legacy): 1,184,569 screp commands, each with exactly one result, all linked; 221,520 selections compared; no failures. On all 524 replays of `scmapanalyzer/replays` (323 legacy, 201 SC:R): 8,559,260 screp commands, one after the end frame and each other one with exactly one result; 1,851,250 selections compared; no failures.

## Data

The engine needs the Blizzard game data files, which are not in this repository. See [openbw-go](https://github.com/marianogappa/openbw-go#game-data). Tests read them from `$OPENBW_DATA` (default `testdata/bwdata`) and replays from `$OPENBW_REPLAYS` (default `testdata/replays`). Both defaults are git-ignored. Tests skip when the files are missing.

## Development

openbw-go is not published yet. `go.mod` has a `replace` directive to a sibling checkout:

```
replace github.com/marianogappa/openbw-go => ../openbw-go
```

Clone openbw-go next to this repository. Remove the directive when openbw-go has a release.

## Licence

MIT. openbw-go is LGPL-3.0.
