package commands

import (
	"context"
	"fmt"

	dg "github.com/bwmarrin/discordgo"
	"github.com/glotchimo/recast/internal/handlers"
	rp "github.com/glotchimo/recast/internal/response"
)

type Ping struct{}

func (p *Ping) Metadata() dg.ApplicationCommand {
	return dg.ApplicationCommand{
		Name:        "ping",
		Description: "Ping the backend",
	}
}

func (p *Ping) Handle(ctx context.Context, dep handlers.Dependencies) error {
	if err := dep.Responder.Defer(dep.Interaction, true); err != nil {
		return err
	}

	embed := dg.MessageEmbed{
		Title:       "Pong!",
		Description: fmt.Sprintf("Latency: %s", dep.Session.HeartbeatLatency()),
	}

	return dep.Responder.Send(dep.Interaction, rp.MessageOptions{Embeds: []*dg.MessageEmbed{&embed}, Ephemeral: true})
}
