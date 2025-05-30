package utils

import (
	"bytes"
	"os/exec"
	"strings"

	dg "github.com/bwmarrin/discordgo"
	"github.com/rs/xid"
)

func GenerateID() string {
	return xid.New().String()
}

func MapOptions(i *dg.InteractionCreate) map[string]*dg.ApplicationCommandInteractionDataOption {
	os := i.ApplicationCommandData().Options
	om := make(map[string]*dg.ApplicationCommandInteractionDataOption, len(os))
	for _, opt := range os {
		om[opt.Name] = opt
	}
	return om
}

func GetCommit() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}

	return strings.TrimSpace(out.String())
}
