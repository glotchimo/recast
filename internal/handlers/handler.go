package handlers

import (
	"context"
	"log/slog"

	dg "github.com/bwmarrin/discordgo"
	ch "github.com/glotchimo/recast/internal/cache"
	db "github.com/glotchimo/recast/internal/database"
	md "github.com/glotchimo/recast/internal/models"
	rp "github.com/glotchimo/recast/internal/response"
)

type Dependencies struct {
	Session     *dg.Session
	Database    *db.Database
	Cache       *ch.Cache
	Responder   *rp.Responder
	Logger      *slog.Logger
	Guild       *md.Guild
	Interaction *dg.InteractionCreate
	Options     *map[string]*dg.ApplicationCommandInteractionDataOption
}

type Handler interface {
	Metadata() dg.ApplicationCommand
	Handle(context.Context, Dependencies) error
}
