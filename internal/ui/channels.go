package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/basecamp/once/internal/docker"
)

type channelChip struct {
	channel docker.Channel
	label   string
	color   color.Color
}

func newChannelChip(channel docker.Channel) channelChip {
	return channelChip{
		channel: channel,
		label:   channelLabel(channel),
		color:   channelColor(channel),
	}
}

func renderChannelChips(chips []channelChip) string {
	parts := make([]string, len(chips))
	for i, chip := range chips {
		parts[i] = lipgloss.NewStyle().Foreground(Colors.Border).Render("[") +
			lipgloss.NewStyle().Foreground(chip.color).Render(chip.label) +
			lipgloss.NewStyle().Foreground(Colors.Border).Render("]")
	}
	return strings.Join(parts, " ")
}

// Helpers

func channelLabel(channel docker.Channel) string {
	switch channel {
	case docker.ChannelNoBackup:
		return "no backup"
	case docker.ChannelIdle:
		return "idle 30d"
	default:
		return string(channel)
	}
}

func channelColor(channel docker.Channel) color.Color {
	switch channel {
	case docker.ChannelRunning:
		return Colors.Success
	case docker.ChannelDeploying:
		return Colors.Focused
	case docker.ChannelStale, docker.ChannelNoBackup:
		return Colors.Warning
	case docker.ChannelUnhealthy:
		return Colors.Error
	case docker.ChannelIdle:
		return Colors.Border
	default:
		return Colors.LightText
	}
}
