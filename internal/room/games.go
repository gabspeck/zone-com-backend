package room

import "fmt"

type GameKind string

const (
	GameKindCheckers   GameKind = "checkers"
	GameKindReversi    GameKind = "reversi"
	GameKindBackgammon GameKind = "backgammon"
)

type GameSession interface {
	MessageName(msgType uint32) string
	HandleMessage(table *Table, p *Player, msgType uint32, payload []byte) error
}

type GameDefinition struct {
	Kind       GameKind
	Name       string
	Service    string
	Seats      int16
	NewSession func() GameSession
}

var gameRegistry = map[string]*GameDefinition{
	"mchkr_zm_***": {
		Kind:       GameKindCheckers,
		Name:       "Checkers",
		Service:    "mchkr_zm_***",
		Seats:      2,
		NewSession: newCheckersSession,
	},
	"mrvse_zm_***": {
		Kind:       GameKindReversi,
		Name:       "Reversi",
		Service:    "mrvse_zm_***",
		Seats:      2,
		NewSession: newReversiSession,
	},
	"mbckg_zm_***": {
		Kind:       GameKindBackgammon,
		Name:       "Backgammon",
		Service:    "mbckg_zm_***",
		Seats:      2,
		NewSession: newBackgammonSession,
	},
}

func ResolveGameDefinition(service string) (*GameDefinition, error) {
	def, ok := gameRegistry[service]
	if !ok {
		return nil, fmt.Errorf("unsupported service %q", service)
	}
	return def, nil
}
