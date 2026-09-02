package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

const maximumConfigFileBytes = 1 << 20

// LoadConfigFile loads a flat YAML or JSON object whose keys match OwlMail
// command-line option names. Unknown, duplicate, nested, and null values fail
// closed so misspelled production settings cannot be silently ignored.
func LoadConfigFile(path string) (*Config, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat config file: %w", err)
	}
	if info.Size() > maximumConfigFileBytes {
		return nil, fmt.Errorf("config file exceeds %d bytes", maximumConfigFileBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config file must contain one flat object")
	}

	fs := flag.NewFlagSet("config-file", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	refs := DefineFlags(fs)
	root := document.Content[0]
	seen := make(map[string]struct{}, len(root.Content)/2)
	for index := 0; index < len(root.Content); index += 2 {
		keyNode, valueNode := root.Content[index], root.Content[index+1]
		key := strings.TrimSpace(keyNode.Value)
		if key == "" || key == "config" {
			return nil, fmt.Errorf("invalid config file key %q", key)
		}
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate config file key %q", key)
		}
		seen[key] = struct{}{}
		if fs.Lookup(key) == nil {
			return nil, fmt.Errorf("unknown config file key %q", key)
		}
		if valueNode.Kind != yaml.ScalarNode || valueNode.Tag == "!!null" {
			return nil, fmt.Errorf("config file key %q must have a scalar value", key)
		}
		value := valueNode.Value
		if valueNode.Tag == "!!bool" {
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("config file key %q: %w", key, err)
			}
			value = strconv.FormatBool(parsed)
		}
		if err := fs.Set(key, value); err != nil {
			return nil, fmt.Errorf("config file key %q: %w", key, err)
		}
	}
	cfg := ResolveConfig(fs, refs)
	cfg.ConfigFile = path
	return cfg, nil
}

func configFileFromArgs(args []string) (string, error) {
	path := strings.TrimSpace(os.Getenv("OWLMAIL_CONFIG_FILE"))
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if argument == "-config" || argument == "--config" {
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return "", fmt.Errorf("%s requires a path", argument)
			}
			path = args[index+1]
			index++
			continue
		}
		if strings.HasPrefix(argument, "-config=") {
			path = strings.TrimPrefix(argument, "-config=")
		}
		if strings.HasPrefix(argument, "--config=") {
			path = strings.TrimPrefix(argument, "--config=")
		}
	}
	return strings.TrimSpace(path), nil
}
