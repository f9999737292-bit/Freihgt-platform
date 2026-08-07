package controltowerreadmodel

import (
	"fmt"
	"strings"
)

type Mode string

const (
	ModeDisabled Mode = "disabled"
	ModeShadow   Mode = "shadow"
	ModePrimary  Mode = "primary"
)

func ParseMode(raw string) (Mode, error) {
	switch Mode(strings.ToLower(strings.TrimSpace(raw))) {
	case "", ModeDisabled:
		return ModeDisabled, nil
	case ModeShadow:
		return ModeShadow, nil
	case ModePrimary:
		return ModePrimary, nil
	default:
		return "", fmt.Errorf("unknown CONTROL_TOWER_READ_MODEL_MODE %q", raw)
	}
}

func (m Mode) Enabled() bool {
	return m == ModeShadow || m == ModePrimary
}
