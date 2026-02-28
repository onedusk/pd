package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProjectConfig holds project-level settings loaded from decompose.yml.
type ProjectConfig struct {
	OutputDir     string   `yaml:"outputDir,omitempty"`
	Languages     []string `yaml:"languages,omitempty"`
	ExcludeDirs   []string `yaml:"excludeDirs,omitempty"`
	TemplatePath  string   `yaml:"templatePath,omitempty"`
	Verbose       bool     `yaml:"verbose,omitempty"`
	SingleAgent   bool     `yaml:"singleAgent,omitempty"`
	GraphExcludes []string `yaml:"graphExcludes,omitempty"`
}

// Load attempts to read decompose.yml or decompose.yaml from the given
// directory. Returns a zero-value config (not an error) if no config file
// exists.
func Load(dir string) (*ProjectConfig, error) {
	for _, name := range []string{"decompose.yml", "decompose.yaml"} {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg ProjectConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
		return &cfg, nil
	}
	return &ProjectConfig{}, nil
}
