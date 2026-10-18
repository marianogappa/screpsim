package screpsim

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/icza/screp/rep/repcmd"
	openbw "github.com/marianogappa/openbw-go"
)

var testReplays = []string{
	"cwalggdl/replays-cwal-dl/30-NEW-lillljilililili/MM-CB157510-11C2-11F1-A3DD-FA167B5461B6.rep",
	"cwalggdl/replays-cwal-dl/30-NEW-Horang2[._.]/MM-39C48C10-35DD-11F1-A3DD-FA167B5461B6.rep",
	"scmapanalyzer/replays/2v2v2v2 BGH !! Favourite!!!.rep",
	"scmapanalyzer/replays/2486 LimperT qeywro78Z.rep",
}

const testMaxFrame = 24 * 60 * 10

var testDataOnce = sync.OnceValues(func() (*openbw.Data, error) {
	dir := os.Getenv("OPENBW_DATA")
	if dir == "" {
		dir = "testdata/bwdata"
	}
	return openbw.LoadData(dir)
})

func testData(t *testing.T) *openbw.Data {
	t.Helper()
	data, err := testDataOnce()
	if err != nil {
		t.Skipf("no game data: %v", err)
	}
	return data
}

func readReplay(t *testing.T, name string) []byte {
	t.Helper()
	dir := os.Getenv("OPENBW_REPLAYS")
	if dir == "" {
		dir = "testdata/replays"
	}
	replay, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Skipf("no replay: %v", err)
	}
	return replay
}

func TestCommandResultsJoinScrepCommands(t *testing.T) {
	data := testData(t)
	for _, name := range testReplays {
		t.Run(filepath.Base(name), func(t *testing.T) {
			t.Parallel()
			replay := readReplay(t, name)
			g, err := Open(replay, Options{Engine: openbw.Options{Data: data, Kinds: openbw.KindCommandResult | openbw.KindUnitCreated, MaxFrame: testMaxFrame}})
			if err != nil {
				t.Fatal(err)
			}
			cmds := g.Replay().Commands.Cmds
			links := g.CommandLinks()
			if len(links) != len(cmds) {
				t.Fatalf("%d links for %d commands", len(links), len(cmds))
			}
			last, results, joined := -1, 0, 0
			for e := range g.Events() {
				if _, raw := e.(*openbw.CommandResult); raw {
					t.Fatal("raw openbw.CommandResult in the stream")
				}
				r, ok := e.(*CommandResult)
				if !ok {
					continue
				}
				results++
				if r.CmdIndex < 0 {
					continue
				}
				joined++
				base := r.Cmd.BaseCmd()
				if r.CmdIndex <= last || r.Cmd != cmds[r.CmdIndex] || int32(base.Frame) != r.Frame || g.Player(r.Owner).ID != base.PlayerID || links[r.CmdIndex].Action != r.Action {
					t.Fatalf("action %d at frame %d slot %d joins command %d after %d: %+v", r.Action, r.Frame, r.Owner, r.CmdIndex, last, base)
				}
				last = r.CmdIndex
			}
			if err := g.Err(); err != nil {
				t.Fatal(err)
			}
			if results == 0 || joined != results {
				t.Fatalf("%d of %d results joined", joined, results)
			}
		})
	}
}

func TestPlayerMapsSlots(t *testing.T) {
	g, err := Open(readReplay(t, testReplays[2]), Options{Engine: openbw.Options{Data: testData(t), Kinds: openbw.NoKinds}})
	if err != nil {
		t.Fatal(err)
	}
	h := g.Engine().Header()
	for _, owner := range g.Engine().PlayingSlots() {
		p := g.Player(owner)
		if p == nil || p.SlotID != uint16(owner) || int32(p.ID) != h.Slots[owner].PlayerID {
			t.Errorf("slot %d (player ID %d): screp player %+v", owner, h.Slots[owner].PlayerID, p)
		}
	}
	if g.Player(-1) != nil || g.Player(11) != nil {
		t.Error("player for slot -1 or 11")
	}
}

func TestUnitTagsMatchSelections(t *testing.T) {
	data := testData(t)
	for _, name := range []string{testReplays[0], testReplays[3]} {
		g, err := Open(readReplay(t, name), Options{Engine: openbw.Options{Data: data, Kinds: openbw.KindCommandResult, MaxFrame: testMaxFrame}})
		if err != nil {
			t.Fatal(err)
		}
		compared := 0
		for e := range g.Events() {
			r, ok := e.(*CommandResult)
			if !ok || r.MissingTags > 0 || r.ForeignTags > 0 {
				continue
			}
			sel, ok := r.Cmd.(*repcmd.SelectCmd)
			if !ok || sel.Type.ID != repcmd.TypeIDSelect && sel.Type.ID != repcmd.TypeIDSelect121 || len(sel.UnitTags) != len(r.Selection) {
				continue
			}
			for _, id := range r.Selection {
				tag := g.UnitTag(id)
				found := false
				for _, want := range sel.UnitTags {
					found = found || want == tag
				}
				if !found {
					t.Fatalf("%s: action %d: unit %d has tag %#x, screp tags %v", name, r.Action, id, tag, sel.UnitTags)
				}
			}
			compared++
		}
		if compared == 0 {
			t.Errorf("%s: no selection compared", name)
		}
	}
	if g := (&Game{}); g.UnitTag(0) != 0 {
		t.Error("tag of no unit")
	}
}

func TestUnitTagLayout(t *testing.T) {
	id := openbw.UnitID(1448 | 1<<11)
	cases := []struct {
		info openbw.EngineInfo
		want repcmd.UnitTag
	}{
		{openbw.EngineInfo{SourceFormat: format121, UnitLimit: 3400}, 3148 | 1<<13},
		{openbw.EngineInfo{SourceFormat: format121, UnitLimit: 1700}, 0x0DA8},
		{openbw.EngineInfo{SourceFormat: "legacy", UnitLimit: 1700}, 0x0DA8},
	}
	for _, c := range cases {
		g := &Game{scrTags: usesSCRTagLayout(c.info)}
		if got := g.UnitTag(id); got != c.want {
			t.Errorf("%s with %d units: tag %#x, want %#x", c.info.SourceFormat, c.info.UnitLimit, got, c.want)
		}
	}
}

func TestParseIsQuiet(t *testing.T) {
	replay := readReplay(t, testReplays[0])
	var out bytes.Buffer
	flags, prefix, writer := log.Flags(), log.Prefix(), log.Writer()
	log.SetOutput(&out)
	defer func() { log.SetOutput(writer); log.SetFlags(flags); log.SetPrefix(prefix) }()
	Open(replay[:len(replay)*2/3], Options{Engine: openbw.Options{Data: testData(t)}})
	if out.Len() != 0 {
		t.Fatalf("screp logged %q", out.String())
	}
}
