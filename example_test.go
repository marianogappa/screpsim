package screpsim_test

import (
	"fmt"
	"log"
	"os"

	openbw "github.com/marianogappa/openbw-go"
	"github.com/marianogappa/screpsim"
)

// Print the commands that the engine rejected, as screp shows them.
func Example() {
	replay, err := os.ReadFile("game.rep")
	if err != nil {
		log.Fatal(err)
	}
	g, err := screpsim.Open(replay, screpsim.Options{Engine: openbw.Options{Kinds: openbw.KindCommandResult}})
	if err != nil {
		log.Fatal(err)
	}
	for e := range g.Events() {
		r, ok := e.(*screpsim.CommandResult)
		if !ok || r.Outcome != openbw.Rejected || r.Cmd == nil {
			continue
		}
		fmt.Println(r.CmdIndex, g.Player(r.Owner).Name, r.Cmd.BaseCmd().Type.Name, r.Cmd.Params(false), r.Rejections)
	}
	if err := g.Err(); err != nil {
		log.Fatal(err)
	}
}
