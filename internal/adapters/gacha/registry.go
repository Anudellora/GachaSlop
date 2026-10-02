package gacha

import (
	"gachaslop/internal/domain"
	"gachaslop/internal/ports"
)

type Registry map[domain.Game]ports.PullSource

func (r Registry) Source(game domain.Game) (ports.PullSource, bool) {
	source, ok := r[game]
	return source, ok
}
