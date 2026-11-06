package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/basecamp/once/internal/docker"
	"github.com/basecamp/once/internal/mouse"
)

const channelTargetPrefix = "channel:"

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

func renderChannelChips(chips []channelChip, active docker.Channel) string {
	parts := make([]string, len(chips))
	for i, chip := range chips {
		borderColor := Colors.Border
		labelStyle := lipgloss.NewStyle().Foreground(chip.color)
		if chip.channel == active {
			borderColor = Colors.Focused
			labelStyle = labelStyle.Bold(true)
		}

		rendered := lipgloss.NewStyle().Foreground(borderColor).Render("[") +
			labelStyle.Render(chip.label) +
			lipgloss.NewStyle().Foreground(borderColor).Render("]")
		parts[i] = mouse.Mark(channelTarget(chip.channel), rendered)
	}
	return strings.Join(parts, " ")
}

// Helpers

func channelTarget(channel docker.Channel) string {
	return channelTargetPrefix + string(channel)
}

func channelFromTarget(target string) (docker.Channel, bool) {
	name, ok := strings.CutPrefix(target, channelTargetPrefix)
	if !ok || name == "" {
		return "", false
	}
	return docker.Channel(name), true
}

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
