package models

import (
	"encoding/json"
	"time"

	dg "github.com/bwmarrin/discordgo"
)

type Interaction struct {
	Interaction *dg.Interaction
	Created     time.Time
}

func (i Interaction) Map() map[string]any {
	ib, _ := json.Marshal(i.Interaction)
	return map[string]any{
		"interaction": ib,
	}
}

func (i Interaction) Table() Table {
	return TableInteractions
}
