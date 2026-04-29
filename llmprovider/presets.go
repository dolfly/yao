package llmprovider

import (
	_ "embed"

	"gopkg.in/yaml.v3"
)

//go:embed presets.yml
var presetsYAML []byte

var presets []ProviderPreset

func init() {
	presets = loadPresets()
}

func loadPresets() []ProviderPreset {
	var list []ProviderPreset
	if err := yaml.Unmarshal(presetsYAML, &list); err != nil {
		panic("llmprovider: failed to parse presets.yml: " + err.Error())
	}
	return list
}

// GetPresets returns a copy of the embedded preset list.
func GetPresets() []ProviderPreset {
	out := make([]ProviderPreset, len(presets))
	copy(out, presets)
	return out
}

// GetPreset returns the preset for the given key, or nil if not found.
func GetPreset(key string) *ProviderPreset {
	for i := range presets {
		if presets[i].Key == key {
			cp := presets[i]
			return &cp
		}
	}
	return nil
}
