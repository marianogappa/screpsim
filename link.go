package screpsim

import (
	"github.com/icza/screp/rep"
	"github.com/icza/screp/rep/repcmd"
	openbw "github.com/marianogappa/openbw-go"
)

func commandIDMatches(sourceID, screpID byte) bool {
	return sourceID == screpID || sourceID == repcmd.TypeIDBuild && screpID == repcmd.VirtualTypeIDLand
}

func linkCommands(cs *rep.Commands, sources []openbw.SourceCommand) (links []CommandLink, actions []int) {
	for _, s := range sources {
		if int(s.Action) >= len(actions) {
			actions = append(actions, make([]int, int(s.Action)+1-len(actions))...)
		}
	}
	for i := range actions {
		actions[i] = -1
	}
	if cs == nil {
		return nil, actions
	}
	links = make([]CommandLink, len(cs.Cmds))
	for i := range links {
		links[i] = CommandLink{Source: -1, Action: -1}
	}
	byFrame := map[int32][]int{}
	for i, c := range cs.Cmds {
		f := int32(c.BaseCmd().Frame)
		byFrame[f] = append(byFrame[f], i)
	}
	cutoff := parseErrorCutoffs(cs, byFrame)
	positions := map[int32]int{}
	stopped := map[int32]bool{}
	for i, s := range sources {
		f := s.Frame
		position := positions[f]
		positions[f] = position + 1
		if stopped[f] {
			continue
		}
		frameCmds := byFrame[f]
		limit, hasLimit := cutoff[f]
		if s.EndsBlock || hasLimit && position >= limit || position >= len(frameCmds) {
			stopped[f] = true
			continue
		}
		index := frameCmds[position]
		base := cs.Cmds[index].BaseCmd()
		if base.PlayerID != s.PlayerID || base.Type == nil || !commandIDMatches(s.ID, base.Type.ID) {
			stopped[f] = true
			continue
		}
		links[index] = CommandLink{Source: int32(i), Action: s.Action}
		if s.Action >= 0 {
			actions[s.Action] = index
		}
	}
	return links, actions
}

func parseErrorCutoffs(cs *rep.Commands, byFrame map[int32][]int) map[int32]int {
	cutoff := map[int32]int{}
	if len(cs.ParseErrCmds) == 0 {
		return cutoff
	}
	index := make(map[repcmd.Cmd]int, len(cs.Cmds))
	for i, c := range cs.Cmds {
		index[c] = i
	}
	for _, pec := range cs.ParseErrCmds {
		f := int32(pec.Frame)
		prev := -1
		if pec.PrevCmd != nil {
			if i, ok := index[pec.PrevCmd]; ok {
				prev = i
			}
		}
		n := 0
		for _, i := range byFrame[f] {
			if i <= prev {
				n++
			}
		}
		if old, ok := cutoff[f]; !ok || n < old {
			cutoff[f] = n
		}
	}
	return cutoff
}
