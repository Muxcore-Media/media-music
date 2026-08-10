package internal

import (
	"fmt"

	"github.com/Muxcore-Media/core/pkg/contracts"
)

func (m *Module) Settings() []contracts.SettingDef {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	return []contracts.SettingDef{{
		Key: "library_dir", Label: "Library directory", Type: contracts.SettingTypeString,
		Value: m.libraryDir, Description: "Root path for music library", Group: "Library",
	}}
}

func (m *Module) UpdateSetting(key, value string) error {
	m.cfgMu.Lock()
	defer m.cfgMu.Unlock()
	switch key {
	case "library_dir":
		m.libraryDir = value
	default:
		return fmt.Errorf("unknown setting %q", key)
	}
	return nil
}
