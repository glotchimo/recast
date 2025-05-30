package bot

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"time"

	sq "github.com/Masterminds/squirrel"
	dg "github.com/bwmarrin/discordgo"
	"github.com/glotchimo/recast/internal/cache"
	"github.com/glotchimo/recast/internal/database"
	"github.com/glotchimo/recast/internal/handlers"
	"github.com/glotchimo/recast/internal/handlers/commands"
	"github.com/glotchimo/recast/internal/models"
	"github.com/glotchimo/recast/internal/response"
	"github.com/glotchimo/recast/internal/utils"
	"github.com/graxinc/errutil"
)

var lookup map[string]handlers.Handler = map[string]handlers.Handler{
	"ping": &commands.Ping{},
}

type EventType int

const (
	EventTypeGuildUpdate EventType = iota
	EventTypeInteraction
	EventTypeMsgDeletion
	EventTypeVoiceUpdate
)

type GuildEvent struct {
	Type EventType

	GuildUpdate *dg.GuildUpdate
	Interaction *dg.InteractionCreate
	MsgDeletion *dg.MessageDelete
	VoiceUpdate *dg.VoiceStateUpdate
}

type GuildContext struct {
	Context context.Context
	Cancel  context.CancelFunc
	Events  chan GuildEvent
	Relay   chan string
}

type Bot struct {
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	s *dg.Session
	d *database.Database
	c *cache.Cache
	l *slog.Logger
	r *response.Responder

	events   chan GuildEvent
	contexts map[string]*GuildContext
}

func NewBot(debug bool, dbURL, cacheURL, token string, shardID, shardCount, intents int) (*Bot, error) {
	b := Bot{
		events:   make(chan GuildEvent),
		contexts: make(map[string]*GuildContext),
	}

	ctx, cancel := context.WithCancel(context.Background())
	b.ctx = ctx
	b.cancel = cancel

	if debug {
		b.l = slog.Default()
	} else {
		b.l = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: true}))
	}

	database, err := database.NewDatabase(b.l, dbURL)
	if err != nil {
		return nil, errutil.With(err)
	}
	b.d = database

	session, err := dg.New("Bot " + token)
	if err != nil {
		return nil, errutil.With(err)
	}
	b.s = session

	b.s.Identify.Intents = dg.Intent(intents)

	b.s.ShardID = shardID
	b.s.ShardCount = shardCount
	b.l.Info("sharding enabled", "shard_id", shardID, "shard_count", shardCount)

	cache, err := cache.NewCache(cacheURL, session, b.l, database)
	if err != nil {
		return nil, errutil.With(err)
	}
	b.c = cache

	b.r = response.NewSessionResponder(b.s, b.l, b.d, b.ctx)

	b.s.AddHandler(func(s *dg.Session, r *dg.Ready) {
		b.l.Info("bot connected to gateway",
			"bot", fmt.Sprintf("%s#%s", r.User.Username, r.User.Discriminator),
			"guilds", len(s.State.Guilds),
			"version", utils.GetCommit(),
			"shard_id", shardID,
			"shard_count", shardCount,
		)
	})

	if err := b.s.Open(); err != nil {
		return nil, errutil.With(err)
	}

	b.s.AddHandler(func(s *dg.Session, g *dg.GuildCreate) { b.register(g.Guild) })
	b.s.AddHandler(func(s *dg.Session, g *dg.GuildDelete) { b.remove(g.Guild) })

	b.s.AddHandler(func(s *dg.Session, i *dg.InteractionCreate) {
		b.enqueue(i.GuildID, GuildEvent{Type: EventTypeInteraction, Interaction: i})
	})
	b.s.AddHandler(func(s *dg.Session, d *dg.MessageDelete) {
		b.enqueue(d.GuildID, GuildEvent{Type: EventTypeMsgDeletion, MsgDeletion: d})
	})
	b.s.AddHandler(func(s *dg.Session, v *dg.VoiceStateUpdate) {
		b.enqueue(v.GuildID, GuildEvent{Type: EventTypeVoiceUpdate, VoiceUpdate: v})
	})

	go b.route()
	go b.status()

	return &b, nil
}

func (b *Bot) Close() {
	defer b.s.Close()
	defer b.d.Close()
	defer b.c.Close()

	b.cancel()
	close(b.events)
}

func (b *Bot) route() {
	for {
		select {
		case <-b.ctx.Done():
			return
		case e := <-b.events:
			if e.Interaction != nil {
				if err := b.d.Create(b.ctx, models.Interaction{
					Interaction: e.Interaction.Interaction,
				}); err != nil {
					b.l.Error("error storing interaction", "error", err)
				}
			}

			switch e.Type {
			case EventTypeGuildUpdate:
				b.register(e.GuildUpdate.Guild)
			case EventTypeInteraction:
				if ctx, ok := b.contexts[e.Interaction.GuildID]; ok {
					ctx.Events <- e
				}
			}
		}
	}
}

func (b *Bot) status() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	s := 0
	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			var msg string
			switch s {
			case 0:
				count, err := b.d.Count(b.ctx, models.TableGuilds, nil)
				if err != nil {
					b.l.Error("error counting all known users", "error", err)
					continue
				}
				msg = fmt.Sprintf("Helping %d servers", count)

			case 1:
				count, err := b.d.Count(b.ctx, models.TableInteractions, nil)
				if err != nil {
					b.l.Error("error counting activities", "error", err)
					continue
				}
				msg = fmt.Sprintf("%d commands handled", count)

			default:
				s = -1
			}

			if err := b.s.UpdateStatusComplex(dg.UpdateStatusData{
				Status: string(dg.StatusOnline),
				Activities: []*dg.Activity{
					{
						Name:  b.s.State.User.Username,
						Type:  dg.ActivityTypeCustom,
						State: msg,
					},
				},
			}); err != nil {
				b.l.Error("error setting bot status", "error", err)
			}

			s++
		}
	}
}

func (b *Bot) ensure(guildID string) *GuildContext {
	b.mu.RLock()
	if guildCtx, exists := b.contexts[guildID]; exists {
		b.mu.RUnlock()
		return guildCtx
	}
	b.mu.RUnlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	if guildCtx, exists := b.contexts[guildID]; exists {
		return guildCtx
	}

	ctx, cancel := context.WithCancel(b.ctx)
	guildCtx := &GuildContext{
		Context: ctx,
		Cancel:  cancel,
		Events:  make(chan GuildEvent, 1000),
		Relay:   make(chan string, 500),
	}

	b.contexts[guildID] = guildCtx
	return guildCtx
}

func (b *Bot) dispatch(guildID string) {
	defer func() {
		if r := recover(); r != nil {
			stack := make([]byte, 4096)
			stack = stack[:runtime.Stack(stack, false)]
			b.l.Error("panic recovered", "guild", guildID, "recovered", r, "stack", stack)
			go b.dispatch(guildID)
		}
	}()

	ctx := b.ensure(guildID)

	g, err := b.d.GetGuild(b.ctx, guildID)
	if err != nil {
		b.l.Error("error getting guild", "guild", guildID, "error", err)
	}

	go b.monitor(guildID, ctx)

	for {
		select {
		case <-ctx.Context.Done():
			return
		case e := <-ctx.Events:
			gc, ok := b.contexts[guildID]
			if !ok {
				b.l.Error("missing guild context entry", "guild", guildID)
				continue
			}

			if e.Interaction != nil {
				if err := b.d.Create(b.ctx, models.Interaction{
					Interaction: e.Interaction.Interaction,
				}); err != nil {
					b.l.Warn("error storing interaction", "error", err)
				}
			}

			switch e.Type {
			case EventTypeInteraction:
				g, err = b.d.GetGuild(b.ctx, guildID)
				if err != nil {
					b.r.Fail(e.Interaction, utils.Failure{
						Type:    utils.ErrInternal,
						Message: "Failed to fetch guild",
						Data:    map[string]any{"error": err, "guild": e.Interaction.GuildID},
					})
					continue
				}

				i := e.Interaction
				if i == nil {
					b.l.Warn("received nil interaction in dispatch")
					continue
				}

				switch i.Type {
				case dg.InteractionApplicationCommand:
					data := i.ApplicationCommandData()
					opts := utils.MapOptions(i)

					h, ok := lookup[data.Name]
					if !ok {
						b.r.Fail(i, utils.Failure{
							Type:    utils.ErrNotFound,
							Message: "No registered command",
						})
						continue
					}

					b.l.Info("command issued", "user", i.Member.User.Username, "called", utils.FormatInteraction(b.s, i))

					go func() {
						defer func() {
							if r := recover(); r != nil {
								stack := make([]byte, 4096)
								stack = stack[:runtime.Stack(stack, false)]
								b.l.Error("panic recovered", "command", h.Metadata().Name, "guild", guildID, "recovered", r, "stack", stack)
							}
						}()

						if err := h.Handle(gc.Context, handlers.Dependencies{
							Session:     b.s,
							Database:    b.d,
							Cache:       b.c,
							Responder:   b.r,
							Logger:      b.l,
							Guild:       g,
							Interaction: i,
							Options:     &opts,
						}); err != nil {
							b.l.Error("error handling command", "error", err, "command", data.Name, "guild", guildID)
							b.r.Fail(i, utils.Failure{
								Type:    utils.ErrInternal,
								Message: "Failed to handle command",
								Data:    map[string]any{"error": err},
							})
						}
					}()

				case dg.InteractionMessageComponent:
				}
			}
		}
	}
}

func (b *Bot) load(guildID string) {
	b.ensure(guildID)

	start := time.Now()

	g, err := b.d.GetGuild(b.ctx, guildID)
	if err != nil {
		b.l.Error("error getting guild", "error", err, "guild", guildID)
		return
	}

	var commands []*dg.ApplicationCommand

	for _, h := range lookup {
		cmd := h.Metadata()
		commands = append(commands, &cmd)
	}

	for i, cmd := range commands {
		result := utils.ValidateCommand(cmd)
		if result.WasModified {
			commands[i] = result.Command
			b.l.Warn("command was modified during validation", "command", cmd.Name, "errors", result.Errors, "guild", guildID)
		}
	}

	var newHash string
	bytes, err := json.Marshal(commands)
	if err == nil {
		hash := sha256.Sum256(bytes)
		newHash = fmt.Sprintf("%x", hash)
	}

	oldHash := g.Settings.CommandSetHash
	if newHash == oldHash {
		b.l.Info("command set unchanged", "guild", guildID)
		return
	}

	if _, err := b.s.ApplicationCommandBulkOverwrite(b.s.State.User.ID, guildID, commands); err != nil {
		b.l.Error("error loading guild commands", "error", err, "guild", guildID)
		return
	}

	if err := b.d.Update(b.ctx, models.TableGuilds, sq.Eq{"id": guildID}, map[string]any{
		"settings": sq.Expr("jsonb_set(COALESCE(settings, '{}'::jsonb), '{command_set_hash}', to_jsonb(?::text))", newHash),
	}); err != nil {
		b.l.Warn("error updating command set hash", "error", err, "guild", guildID, "hash", newHash)
	}

	b.l.Info("command set loaded", "loaded", len(commands), "duration", time.Since(start))
}

func (b *Bot) enqueue(guildID string, event GuildEvent) {
	b.mu.RLock()
	ctx, ok := b.contexts[guildID]
	b.mu.RUnlock()

	if !ok {
		b.l.Warn("attempted to enqueue event for unknown guild", "guild", guildID)
		return
	}

	select {
	case ctx.Events <- event:
	case <-ctx.Context.Done():
		b.l.Debug("dropped event for cancelled guild context", "guild", guildID)
	default:
		b.l.Warn("event channel full, dropping event", "guild", guildID)
	}
}

func (b *Bot) register(g *dg.Guild) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if existing, ok := b.contexts[g.ID]; ok {
		existing.Cancel()
	}

	ctx, cancel := context.WithCancel(b.ctx)

	guildCtx := &GuildContext{
		Context: ctx,
		Cancel:  cancel,
		Events:  make(chan GuildEvent),
		Relay:   make(chan string),
	}

	b.contexts[g.ID] = guildCtx

	stored, err := b.d.GetGuild(b.ctx, g.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if err := b.d.PutGuild(b.ctx, models.Guild{ID: g.ID, Name: g.Name}); err != nil {
				b.l.Error("error storing new guild", "error", err)
				return
			}
		} else {
			b.l.Error("error fetching guild", "error", err)
			return
		}
	} else {
		stored.Name = g.Name
		if err := b.d.Update(b.ctx, models.TableGuilds, sq.Eq{"id": g.ID}, map[string]any{
			"name":     g.Name,
			"settings": sq.Expr("jsonb_set(COALESCE(settings, '{}'::jsonb), '{command_set_hash}', to_jsonb(?::text))", stored.Settings.CommandSetHash),
		}); err != nil {
			b.l.Error("error updating guild", "error", err)
			return
		}
	}

	b.l.Info("registered guild", "id", g.ID, "name", g.Name)

	go b.load(g.ID)
	go b.dispatch(g.ID)
}

func (b *Bot) remove(g *dg.Guild) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if guildCtx, ok := b.contexts[g.ID]; ok {
		guildCtx.Cancel()
		delete(b.contexts, g.ID)
	}

	b.l.Info("removed guild", "id", g.ID)
}

func (b *Bot) monitor(guildID string, ctx *GuildContext) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	var lastWarningTime time.Time
	var consecutiveWarnings int

	for {
		select {
		case <-ctx.Context.Done():
			return
		case <-ticker.C:
			channelSize := 0
			for i := 0; i < 500; i++ {
				select {
				case <-ctx.Events:
					b.l.Error("unexpectedly removed event during channel monitoring", "guild", guildID)
					channelSize++
				default:
					i = 500
				}
			}

			currentLen := len(ctx.Events)
			capacity := cap(ctx.Events)
			fillPercentage := float64(currentLen) / float64(capacity) * 100

			if fillPercentage > 60 {
				now := time.Now()
				if now.Sub(lastWarningTime) > 5*time.Minute {
					consecutiveWarnings = 0
					lastWarningTime = now
				}

				consecutiveWarnings++

				b.l.Warn("event channel filling up",
					"guild", guildID,
					"size", currentLen,
					"capacity", capacity,
					"percentage", fmt.Sprintf("%.1f%%", fillPercentage),
					"consecutive_warnings", consecutiveWarnings)

				if consecutiveWarnings >= 3 {
					b.l.Error("potential stuck handler detected; event channel consistently full",
						"guild", guildID,
						"size", currentLen,
						"capacity", capacity,
						"warnings", consecutiveWarnings)
				}
			}
		}
	}
}
