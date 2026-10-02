package domain

import (
	"fmt"
	"strings"
)

type Game string

const (
	GameGenshin Game = "genshin"
	GameHSR     Game = "hsr"
	GameZZZ     Game = "zzz"
	GameWuWa    Game = "wuwa"
)

func ParseGame(value string) (Game, error) {
	game := Game(strings.ToLower(strings.TrimSpace(value)))
	switch game {
	case GameGenshin, GameHSR, GameZZZ, GameWuWa:
		return game, nil
	default:
		return "", fmt.Errorf("unsupported game %q", value)
	}
}

func (g Game) Valid() bool {
	_, err := ParseGame(string(g))
	return err == nil
}
