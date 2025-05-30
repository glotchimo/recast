package utils

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	dg "github.com/bwmarrin/discordgo"
)

type TimestampType string

const (
	TimestampShort         TimestampType = "t" // e.g., 16:20
	TimestampLong          TimestampType = "T" // e.g., 16:20:30
	TimestampDate          TimestampType = "d" // e.g., 20/04/2021
	TimestampLongDate      TimestampType = "D" // e.g., 20 April 2021
	TimestampShortDateTime TimestampType = "f" // e.g., 20 April 2021 16:20
	TimestampLongDateTime  TimestampType = "F" // e.g., Tuesday, 20 April 2021 16:20
	TimestampRelative      TimestampType = "R" // e.g., 2 months ago
)

func FormatTimestamp(t time.Time, style TimestampType) string {
	timestamp := t.Unix()
	return fmt.Sprintf("<t:%d:%s>", timestamp, style)
}

func FormatMap(m map[string]any) string {
	var b strings.Builder
	formatMapLevel(&b, m, 0)
	return b.String()
}

func formatMapLevel(b *strings.Builder, m map[string]any, level int) {
	indent := strings.Repeat("  ", level)

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := m[k]
		b.WriteString(indent)
		b.WriteString(k)
		b.WriteString(": ")

		switch val := v.(type) {
		case []byte:
			var jsonVal any
			if err := json.Unmarshal(val, &jsonVal); err == nil {
				formatValue(b, jsonVal, level)
			} else {
				b.WriteString(fmt.Sprintf("0x%x", val))
			}
		case []any, []string:
			formatValue(b, val, level)
		case map[string]any:
			b.WriteString("\n")
			formatMapLevel(b, val, level+1)
		default:
			formatValue(b, val, level)
		}
		b.WriteString("\n")
	}
}

func formatValue(b *strings.Builder, v any, level int) {
	indent := strings.Repeat("  ", level+1)

	switch val := v.(type) {
	case []any:
		if len(val) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("\n")
		for _, item := range val {
			b.WriteString(indent)
			b.WriteString("- ")
			formatValue(b, item, level+1)
			b.WriteString("\n")
		}
	case []string:
		if len(val) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("\n")
		for _, item := range val {
			b.WriteString(indent)
			b.WriteString("- ")
			b.WriteString(item)
			b.WriteString("\n")
		}
	case map[string]any:
		formatMapLevel(b, val, level+1)
	default:
		b.WriteString(fmt.Sprintf("%v", v))
	}
}

func FormatDuration(d time.Duration) string {
	d = d.Round(time.Minute)

	years := d / (365 * 24 * time.Hour)
	d -= years * 365 * 24 * time.Hour

	months := d / (30 * 24 * time.Hour)
	d -= months * 30 * 24 * time.Hour

	weeks := d / (7 * 24 * time.Hour)
	d -= weeks * 7 * 24 * time.Hour

	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour

	h := d / time.Hour
	d -= h * time.Hour

	m := d / time.Minute

	parts := []string{}

	if years > 0 {
		if years == 1 {
			parts = append(parts, "1 year")
		} else {
			parts = append(parts, fmt.Sprintf("%d years", years))
		}
	}

	if months > 0 {
		if months == 1 {
			parts = append(parts, "1 month")
		} else {
			parts = append(parts, fmt.Sprintf("%d months", months))
		}
	}

	if weeks > 0 {
		if weeks == 1 {
			parts = append(parts, "1 week")
		} else {
			parts = append(parts, fmt.Sprintf("%d weeks", weeks))
		}
	}

	if days > 0 {
		if days == 1 {
			parts = append(parts, "1 day")
		} else {
			parts = append(parts, fmt.Sprintf("%d days", days))
		}
	}

	if h > 0 {
		if h == 1 {
			parts = append(parts, "1 hour")
		} else {
			parts = append(parts, fmt.Sprintf("%d hours", h))
		}
	}

	if m > 0 {
		if m == 1 {
			parts = append(parts, "1 minute")
		} else {
			parts = append(parts, fmt.Sprintf("%d minutes", m))
		}
	}

	if len(parts) == 0 {
		return "0 minutes"
	}

	if len(parts) == 1 {
		return parts[0]
	}

	if len(parts) == 2 {
		return parts[0] + " and " + parts[1]
	}

	return strings.Join(parts[:len(parts)-1], ", ") + ", and " + parts[len(parts)-1]
}

func FormatUserMention(id string) string {
	return fmt.Sprintf("<@%s>", id)
}

func FormatRoleMention(id string) string {
	return fmt.Sprintf("<@&%s>", id)
}

func FormatInteraction(s *dg.Session, i *dg.InteractionCreate) string {
	if i.Type != dg.InteractionApplicationCommand {
		return ""
	}

	data := i.ApplicationCommandData()
	parts := []string{"/" + data.Name}

	for _, opt := range data.Options {
		parts = append(parts, formatCommandOption(s, i.GuildID, opt))
	}

	return strings.Join(parts, " ")
}

func formatCommandValue(s *dg.Session, guildID string, opt *dg.ApplicationCommandInteractionDataOption) string {
	switch opt.Type {
	case dg.ApplicationCommandOptionString:
		return opt.StringValue()
	case dg.ApplicationCommandOptionInteger:
		return fmt.Sprintf("%d", opt.IntValue())
	case dg.ApplicationCommandOptionBoolean:
		return fmt.Sprintf("%t", opt.BoolValue())
	case dg.ApplicationCommandOptionUser:
		return opt.UserValue(s).Username
	case dg.ApplicationCommandOptionChannel:
		return opt.ChannelValue(s).Name
	case dg.ApplicationCommandOptionRole:
		return opt.RoleValue(s, guildID).Name
	case dg.ApplicationCommandOptionNumber:
		return fmt.Sprintf("%.2f", opt.FloatValue())
	default:
		return fmt.Sprintf("%v", opt.Value)
	}
}

func formatCommandOption(s *dg.Session, guildID string, opt *dg.ApplicationCommandInteractionDataOption) string {
	switch opt.Type {
	case dg.ApplicationCommandOptionSubCommand, dg.ApplicationCommandOptionSubCommandGroup:
		subParts := []string{opt.Name}
		for _, subOpt := range opt.Options {
			subParts = append(subParts, formatCommandOption(s, guildID, subOpt))
		}
		return strings.Join(subParts, " ")
	default:
		return fmt.Sprintf("%s:%v", opt.Name, formatCommandValue(s, guildID, opt))
	}
}
