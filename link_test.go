package screpsim

import (
	"slices"
	"testing"

	"github.com/icza/screp/rep"
	"github.com/icza/screp/rep/repcmd"
	"github.com/icza/screp/rep/repcore"
	openbw "github.com/marianogappa/openbw-go"
)

func screpCmd(frame int32, playerID, id byte) repcmd.Cmd {
	return &repcmd.GeneralCmd{Base: &repcmd.Base{Frame: repcore.Frame(frame), PlayerID: playerID, Type: repcmd.TypeByID(id)}}
}

func source(frame int32, playerID, id byte, action int32) openbw.SourceCommand {
	return openbw.SourceCommand{Frame: frame, PlayerID: playerID, ID: id, Action: action}
}

func linkSources(t *testing.T, cs *rep.Commands, sources []openbw.SourceCommand) []int32 {
	t.Helper()
	links, actions := linkCommands(cs, sources)
	got := make([]int32, len(links))
	for i, l := range links {
		got[i] = l.Source
		if l.Source >= 0 && sources[l.Source].Action >= 0 && actions[sources[l.Source].Action] != i {
			t.Fatalf("action %d maps to %d, want %d", sources[l.Source].Action, actions[sources[l.Source].Action], i)
		}
	}
	return got
}

func TestLinkExact(t *testing.T) {
	cs := &rep.Commands{Cmds: []repcmd.Cmd{screpCmd(1, 0, 0x09), screpCmd(1, 1, 0x14), screpCmd(2, 0, 0x0c)}}
	sources := []openbw.SourceCommand{source(1, 0, 0x09, 0), source(1, 1, 0x14, 1), source(2, 0, 0x0c, 2)}
	if got := linkSources(t, cs, sources); !slices.Equal(got, []int32{0, 1, 2}) {
		t.Fatalf("links %v", got)
	}
}

func TestLinkBuildToLand(t *testing.T) {
	cs := &rep.Commands{Cmds: []repcmd.Cmd{screpCmd(5, 0, repcmd.VirtualTypeIDLand)}}
	sources := []openbw.SourceCommand{source(5, 0, repcmd.TypeIDBuild, 0)}
	if got := linkSources(t, cs, sources); !slices.Equal(got, []int32{0}) {
		t.Fatalf("links %v", got)
	}
}

func TestLinkStopsFrameAtMismatch(t *testing.T) {
	cs := &rep.Commands{Cmds: []repcmd.Cmd{screpCmd(1, 0, 0x09), screpCmd(1, 0, 0x14), screpCmd(1, 0, 0x15), screpCmd(2, 0, 0x09)}}
	sources := []openbw.SourceCommand{source(1, 0, 0x09, 0), source(1, 0, 0x0c, 1), source(1, 0, 0x15, 2), source(2, 0, 0x09, 3)}
	if got := linkSources(t, cs, sources); !slices.Equal(got, []int32{0, -1, -1, 3}) {
		t.Fatalf("links %v", got)
	}
}

func TestLinkStopsFrameAtUnreadableCommand(t *testing.T) {
	cs := &rep.Commands{Cmds: []repcmd.Cmd{screpCmd(1, 0, 0x09), screpCmd(1, 0, 0xd2)}}
	unreadable := source(1, 0, 0xd2, -1)
	unreadable.Dropped, unreadable.EndsBlock = "unknown command", true
	if got := linkSources(t, cs, []openbw.SourceCommand{source(1, 0, 0x09, 0), unreadable}); !slices.Equal(got, []int32{0, -1}) {
		t.Fatalf("links %v", got)
	}
}

func TestLinkKeepsDroppedCommands(t *testing.T) {
	cs := &rep.Commands{Cmds: []repcmd.Cmd{screpCmd(1, 0, 0x37), screpCmd(1, 0, 0x14)}}
	dropped := source(1, 0, 0x37, -1)
	dropped.Dropped = "not read by OpenBW"
	links, actions := linkCommands(cs, []openbw.SourceCommand{dropped, source(1, 0, 0x14, 0)})
	want := []CommandLink{{Source: 0, Action: -1}, {Source: 1, Action: 0}}
	if !slices.Equal(links, want) || !slices.Equal(actions, []int{1}) {
		t.Fatalf("links %v, actions %v", links, actions)
	}
}

func TestLinkStopsAtParseError(t *testing.T) {
	first, second := screpCmd(1, 0, 0x09), screpCmd(1, 0, 0x14)
	cs := &rep.Commands{
		Cmds:         []repcmd.Cmd{first, second},
		ParseErrCmds: []*repcmd.ParseErrCmd{{Base: &repcmd.Base{Frame: 1}, PrevCmd: first}},
	}
	sources := []openbw.SourceCommand{source(1, 0, 0x09, 0), source(1, 0, 0x14, 1)}
	if got := linkSources(t, cs, sources); !slices.Equal(got, []int32{0, -1}) {
		t.Fatalf("links %v", got)
	}
}

func TestLinkWithoutScrepCommands(t *testing.T) {
	links, actions := linkCommands(nil, []openbw.SourceCommand{source(1, 0, 0x09, 0)})
	if links != nil || !slices.Equal(actions, []int{-1}) {
		t.Fatalf("links %v, actions %v", links, actions)
	}
}
