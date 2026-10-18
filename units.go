package screpsim

import (
	"github.com/icza/screp/rep/repcmd"
	openbw "github.com/marianogappa/openbw-go"
)

const (
	format121             = "1.21+"
	firstAllocatedIndex   = 1
	openBWUnitCapacity    = 1700
	scrIndexBits          = 13
	scrTagGenerationLimit = 1 << (16 - scrIndexBits)
)

// usesSCRTagLayout tells if the replay's tags have the 13-bit index of a 3400-unit array.
// An early 1.21 replay without LMTS has a 1700-unit array and keeps the 1.16 layout.
func usesSCRTagLayout(info openbw.EngineInfo) bool {
	return info.SourceFormat == format121 && info.UnitLimit > openBWUnitCapacity
}

// UnitTag is the unit tag that screp reads from this replay's commands for the engine unit id, or 0 for no unit.
// For a 1.21 replay with a 3400-unit array, screp keeps the low 16 bits of the 32-bit tag: the 13-bit index
// and the generation modulo 8. The index mapping is the downgrade's: index 1 stays 1, and the other indexes add 1700.
// Other replays carry the 1.16 tag.
func (g *Game) UnitTag(id openbw.UnitID) repcmd.UnitTag {
	if id == 0 {
		return 0
	}
	if !g.scrTags {
		return repcmd.UnitTag(id.LegacyTag())
	}
	index := uint32(id.Index())
	if index != firstAllocatedIndex {
		index += openBWUnitCapacity
	}
	return repcmd.UnitTag(index | id.Generation()%scrTagGenerationLimit<<scrIndexBits)
}
