package main

import (
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/icza/screp/rep/repcmd"
	openbw "github.com/marianogappa/openbw-go"
	"github.com/marianogappa/screpsim"
)

const maxFailures = 20

type result struct {
	path          string
	format        string
	err           error
	cmds          int
	results       int
	linked        int
	unlinked      int
	dropped       int
	afterEnd      int
	selections    int
	failures      []string
	failureCount  int
	failureByKind map[string]int
}

func (r *result) fail(kind, format string, args ...any) {
	if r.failureByKind == nil {
		r.failureByKind = map[string]int{}
	}
	r.failureByKind[kind]++
	r.failureCount++
	if len(r.failures) < maxFailures {
		r.failures = append(r.failures, kind+": "+fmt.Sprintf(format, args...))
	}
}

type checker struct {
	r       *result
	g       *screpsim.Game
	cmds    []repcmd.Cmd
	links   []screpsim.CommandLink
	sources []openbw.SourceCommand
	header  openbw.Header
	seen    map[int]int
	last    int
}

func checkReplay(data *openbw.Data, path string) *result {
	r := &result{path: path}
	replay, err := os.ReadFile(path)
	if err != nil {
		r.err = err
		return r
	}
	g, err := screpsim.Open(replay, screpsim.Options{Engine: openbw.Options{Data: data, Kinds: openbw.KindCommandResult}})
	if err != nil {
		r.err = err
		return r
	}
	c := &checker{r: r, g: g, links: g.CommandLinks(), sources: g.Engine().SourceCommands(), header: g.Engine().Header(), seen: map[int]int{}, last: -1}
	if g.Replay().Commands != nil {
		c.cmds = g.Replay().Commands.Cmds
	}
	r.format = g.Engine().Info().SourceFormat
	r.cmds = len(c.cmds)
	for e := range g.Events() {
		if res, ok := e.(*screpsim.CommandResult); ok {
			c.result(res)
		}
	}
	if err := g.Err(); err != nil {
		r.err = err
		return r
	}
	c.everyCommand()
	return r
}

func (c *checker) result(res *screpsim.CommandResult) {
	c.r.results++
	if res.CmdIndex < 0 {
		c.r.unlinked++
		return
	}
	c.r.linked++
	i := res.CmdIndex
	if i <= c.last {
		c.r.fail("order", "action %d links to screp command %d after command %d", res.Action, i, c.last)
	}
	c.last = i
	c.seen[i]++
	if res.Cmd != c.cmds[i] {
		c.r.fail("command", "action %d: Cmd is not Commands.Cmds[%d]", res.Action, i)
	}
	base := res.Cmd.BaseCmd()
	p := c.g.Player(res.Owner)
	switch {
	case int32(base.Frame) != res.Frame:
		c.r.fail("frame", "action %d at frame %d links to screp command %d at frame %d", res.Action, res.Frame, i, base.Frame)
	case p == nil || p.ID != base.PlayerID || c.header.Slots[res.Owner].PlayerID != int32(base.PlayerID):
		c.r.fail("player", "action %d of slot %d links to screp command %d of player %d", res.Action, res.Owner, i, base.PlayerID)
	case c.links[i].Action != res.Action || c.links[i].Source != res.Source:
		c.r.fail("link", "action %d source %d, CommandLinks[%d] %+v", res.Action, res.Source, i, c.links[i])
	case !idMatches(c.sources[res.Source].ID, base.Type.ID):
		c.r.fail("type", "action %d source ID %#x, screp type %#x", res.Action, c.sources[res.Source].ID, base.Type.ID)
	}
	c.selection(res)
}

func idMatches(sourceID, screpID byte) bool {
	return sourceID == screpID || sourceID == repcmd.TypeIDBuild && screpID == repcmd.VirtualTypeIDLand
}

func (c *checker) selection(res *screpsim.CommandResult) {
	sel, ok := res.Cmd.(*repcmd.SelectCmd)
	replaces := res.Cmd.BaseCmd().Type.ID == repcmd.TypeIDSelect || res.Cmd.BaseCmd().Type.ID == repcmd.TypeIDSelect121
	if !ok || !replaces || res.MissingTags > 0 || res.ForeignTags > 0 || len(res.Selection) != len(sel.UnitTags) {
		return
	}
	c.r.selections++
	want := map[repcmd.UnitTag]bool{}
	for _, tag := range sel.UnitTags {
		want[tag] = true
	}
	for _, id := range res.Selection {
		if !want[c.g.UnitTag(id)] {
			c.r.fail("unit tag", "action %d: unit %d has tag %#x, screp tags %v", res.Action, id, c.g.UnitTag(id), sel.UnitTags)
			return
		}
	}
}

func (c *checker) everyCommand() {
	end := c.g.Engine().EndFrame()
	for i, l := range c.links {
		n := c.seen[i]
		switch {
		case l.Action >= 0 && int32(c.cmds[i].BaseCmd().Frame) >= end:
			c.r.afterEnd++
			if n != 0 {
				c.r.fail("results", "screp command %d after the end frame has %d results", i, n)
			}
		case l.Action >= 0:
			if n != 1 {
				c.r.fail("results", "screp command %d (action %d) has %d results", i, l.Action, n)
			}
		case l.Source >= 0:
			c.r.dropped++
			if c.sources[l.Source].Dropped == "" || n != 0 {
				c.r.fail("results", "screp command %d: source %d not executed and not dropped, %d results", i, l.Source, n)
			}
		default:
			if n != 0 {
				c.r.fail("results", "unlinked screp command %d has %d results", i, n)
			}
		}
	}
}

func failureKinds(results []*result) []string {
	kinds := map[string]bool{}
	for _, r := range results {
		for k := range r.failureByKind {
			kinds[k] = true
		}
	}
	return slices.Sorted(maps.Keys(kinds))
}
