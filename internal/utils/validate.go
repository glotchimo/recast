package utils

import (
	dg "github.com/bwmarrin/discordgo"
)

const (
	maxCommandNameLength        = 32
	maxCommandDescriptionLength = 100
	maxOptionsPerCommand        = 25
	maxChoicesPerOption         = 25
	maxOptionNameLength         = 32
	maxOptionDescLength         = 100
	maxChoiceNameLength         = 100
	maxChoiceValueLength        = 100
)

type ValidationResult struct {
	Command     *dg.ApplicationCommand
	IsValid     bool
	WasModified bool
	Errors      []string
}

func ValidateCommand(cmd *dg.ApplicationCommand) ValidationResult {
	result := ValidationResult{
		Command: cmd,
		IsValid: true,
	}

	if len(cmd.Name) > maxCommandNameLength {
		result.Command.Name = cmd.Name[:maxCommandNameLength]
		result.WasModified = true
		result.Errors = append(result.Errors, "Command name was truncated")
	}

	if len(cmd.Description) > maxCommandDescriptionLength {
		result.Command.Description = cmd.Description[:maxCommandDescriptionLength]
		result.WasModified = true
		result.Errors = append(result.Errors, "Command description was truncated")
	}

	if len(cmd.Options) > maxOptionsPerCommand {
		result.Command.Options = cmd.Options[:maxOptionsPerCommand]
		result.WasModified = true
		result.Errors = append(result.Errors, "Excess options were removed")
	}

	for i, opt := range result.Command.Options {
		if len(opt.Name) > maxOptionNameLength {
			result.Command.Options[i].Name = opt.Name[:maxOptionNameLength]
			result.WasModified = true
			result.Errors = append(result.Errors, "Option name was truncated")
		}

		if len(opt.Description) > maxOptionDescLength {
			result.Command.Options[i].Description = opt.Description[:maxOptionDescLength]
			result.WasModified = true
			result.Errors = append(result.Errors, "Option description was truncated")
		}

		if len(opt.Choices) > maxChoicesPerOption {
			result.Command.Options[i].Choices = opt.Choices[:maxChoicesPerOption]
			result.WasModified = true
			result.Errors = append(result.Errors, "Excess choices were removed")
		}

		for j, choice := range opt.Choices {
			if len(choice.Name) > maxChoiceNameLength {
				result.Command.Options[i].Choices[j].Name = choice.Name[:maxChoiceNameLength]
				result.WasModified = true
				result.Errors = append(result.Errors, "Choice name was truncated")
			}

			if strVal, ok := choice.Value.(string); ok {
				if len(strVal) > maxChoiceValueLength {
					result.Command.Options[i].Choices[j].Value = strVal[:maxChoiceValueLength]
					result.WasModified = true
					result.Errors = append(result.Errors, "Choice value was truncated")
				}
			}
		}
	}

	return result
}
