package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func wrapBlock(content string, width int) string {
	if width <= 0 {
		return content
	}
	return lipgloss.NewStyle().Width(width).Render(content)
}

func clipToHeight(content string, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) <= height {
		return content
	}
	return strings.Join(lines[:height], "\n")
}

func splitPaneHeights(total int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	bottom := total / 3
	if bottom < 5 {
		bottom = minInt(5, total)
	}
	if total-bottom < 5 {
		bottom = total / 2
	}
	top := total - bottom
	if top < 3 {
		top = total / 2
		bottom = total - top
	}
	return top, bottom
}

func panelWidths(total int) (int, int) {
	left := total / 3
	if left < 24 {
		left = 24
	}
	if left > 40 {
		left = 40
	}
	right := total - left
	if right < 0 {
		right = 0
	}
	return left, right
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
