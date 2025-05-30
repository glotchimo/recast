package models

import (
	"encoding/json"
	"time"
)

type Guild struct {
	ID       string
	Name     string
	Settings struct {
		LogChannelID   string `json:"log_channel_id"`
		CommandSetHash string `json:"command_set_hash"`
	}
	Created time.Time
	Updated time.Time
	Deleted *time.Time
}

func (g Guild) Map() map[string]any {
	settings, _ := json.Marshal(g.Settings)

	return map[string]any{
		"id":       g.ID,
		"name":     g.Name,
		"settings": settings,
		"created":  g.Created,
	}
}

func (g Guild) Table() Table {
	return TableGuilds
}
