package response

import (
	"context"
	"fmt"
	"log/slog"

	dg "github.com/bwmarrin/discordgo"
	"github.com/glotchimo/recast/internal/database"
	"github.com/glotchimo/recast/internal/utils"
)

type MessageOptions struct {
	Content    string
	Embeds     []*dg.MessageEmbed
	Files      []*dg.File
	Components []dg.MessageComponent
	Ephemeral  bool
	Update     bool
	MessageID  string
	ChannelID  string
}

type Responder struct {
	s   *dg.Session
	l   *slog.Logger
	d   *database.Database
	ctx context.Context
}

func NewSessionResponder(s *dg.Session, l *slog.Logger, d *database.Database, ctx context.Context) *Responder {
	return &Responder{
		s:   s,
		l:   l,
		d:   d,
		ctx: ctx,
	}
}

func (r *Responder) Defer(i *dg.InteractionCreate, ephemeral bool) error {
	var err error
	if ephemeral {
		err = r.s.InteractionRespond(i.Interaction, &dg.InteractionResponse{
			Type: dg.InteractionResponseDeferredChannelMessageWithSource,
			Data: &dg.InteractionResponseData{
				Flags: dg.MessageFlagsEphemeral,
			},
		})
	} else {
		err = r.s.InteractionRespond(i.Interaction, &dg.InteractionResponse{
			Type: dg.InteractionResponseDeferredChannelMessageWithSource,
		})
	}

	return err
}

func (r *Responder) Send(i *dg.InteractionCreate, opts MessageOptions) error {
	params := &dg.WebhookParams{
		Content:    opts.Content,
		Embeds:     opts.Embeds,
		Files:      opts.Files,
		Components: opts.Components,
	}

	if opts.Ephemeral {
		params.Flags = dg.MessageFlagsEphemeral
	}

	if opts.Update && opts.MessageID != "" {
		edit := &dg.WebhookEdit{
			Content:    &opts.Content,
			Embeds:     &opts.Embeds,
			Components: &opts.Components,
		}
		_, err := r.s.FollowupMessageEdit(i.Interaction, opts.MessageID, edit)
		return err
	}

	_, err := r.s.FollowupMessageCreate(i.Interaction, true, params)
	return err
}

func (r *Responder) Edit(i *dg.InteractionCreate, opts MessageOptions) error {
	if opts.MessageID == "" {
		return fmt.Errorf("message ID required for edit")
	}

	edit := &dg.WebhookEdit{
		Content:    &opts.Content,
		Embeds:     &opts.Embeds,
		Components: &opts.Components,
	}

	_, err := r.s.FollowupMessageEdit(i.Interaction, opts.MessageID, edit)
	return err
}

func (r *Responder) Delete(i *dg.InteractionCreate, messageID string) error {
	return r.s.FollowupMessageDelete(i.Interaction, messageID)
}

func (r *Responder) Fail(i *dg.InteractionCreate, ctx utils.Failure) error {
	r.l.Warn("handler failure", "type", ctx.Type, "message", ctx.Message, "data", ctx.Data)

	var title, description string
	var color int
	switch ctx.Type {
	case utils.ErrInternal:
		error, ok := ctx.Data["error"]
		if !ok {
			error = "An unexpected error occurred. Our team has been notified."
		}

		str, ok := error.(string)
		if !ok {
			str = fmt.Sprintf("%v", error)
		}

		description = fmt.Sprintf("%s\n\nError Details (please share with support):\n```%s```", ctx.Message, str)
		color = 0xFF0000

	case utils.ErrBadInput:
		title = "Invalid Input"
		description = fmt.Sprintf("%s\n\nDouble-check your input and try again.", ctx.Message)
		color = 0xFFA500

	case utils.ErrNotAllowed:
		title = "Permission Denied"
		description = fmt.Sprintf("%s\n\nIf this doesn't seem right, let an admin know.", ctx.Message)
		color = 0xFF0000

	case utils.ErrNotFound:
		title = "Not Found"
		description = ctx.Message
		color = 0xFFA500

	case utils.ErrTooLarge:
		title = "Response Too Large"
		description = "The output exceeds Discord's message size limit. Try narrowing down your request or breaking it into smaller parts."
		color = 0xFFEF00
	}

	embed := &dg.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       color,
	}

	_, err := r.s.FollowupMessageCreate(i.Interaction, true, &dg.WebhookParams{
		Embeds: []*dg.MessageEmbed{embed},
	})
	return err
}
