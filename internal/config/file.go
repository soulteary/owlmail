package config

import (
	"bytes"
	"flag"
	"fmt"
	"io"
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
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config file: %w", err)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat config file: %w", err)
	}
	if info.Size() > maximumConfigFileBytes {
		return nil, fmt.Errorf("config file exceeds %d bytes", maximumConfigFileBytes)
	}
	data, err := io.ReadAll(io.LimitReader(file, maximumConfigFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	if len(data) > maximumConfigFileBytes {
		return nil, fmt.Errorf("config file exceeds %d bytes", maximumConfigFileBytes)
	}
	var document yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return nil, fmt.Errorf("parse additional config document: %w", err)
		}
		return nil, fmt.Errorf("config file must contain exactly one document")
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
	fs := flag.NewFlagSet("config-file-scan", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	DefineFlags(fs)
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if argument == "--" || argument == "-" || !strings.HasPrefix(argument, "-") {
			break
		}
		nameAndValue := strings.TrimPrefix(argument, "-")
		nameAndValue = strings.TrimPrefix(nameAndValue, "-")
		name, value, hasValue := strings.Cut(nameAndValue, "=")
		option := fs.Lookup(name)
		if option == nil {
			break
		}
		if name == "config" {
			if !hasValue {
				if index+1 >= len(args) {
					return "", fmt.Errorf("%s requires a path", argument)
				}
				index++
				value = args[index]
			}
			path = value
			continue
		}
		if hasValue {
			continue
		}
		if boolean, ok := option.Value.(interface{ IsBoolFlag() bool }); ok && boolean.IsBoolFlag() {
			continue
		}
		if index+1 >= len(args) {
			return "", fmt.Errorf("%s requires a value", argument)
		}
		index++
	}
	return strings.TrimSpace(path), nil
}

// HelpRequested reports whether -h or -help appears where FlagSet.Parse would
// interpret it as an option. Help-looking tokens consumed as values, or those
// after a positional argument or --, are deliberately ignored.
func HelpRequested(args []string, fs *flag.FlagSet) bool {
	if fs == nil {
		return false
	}
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if argument == "--" || argument == "-" || !strings.HasPrefix(argument, "-") {
			return false
		}
		nameAndValue := strings.TrimPrefix(strings.TrimPrefix(argument, "-"), "-")
		name, _, hasValue := strings.Cut(nameAndValue, "=")
		if name == "h" || name == "help" {
			return true
		}
		option := fs.Lookup(name)
		if option == nil {
			return false
		}
		if hasValue {
			continue
		}
		if boolean, ok := option.Value.(interface{ IsBoolFlag() bool }); ok && boolean.IsBoolFlag() {
			continue
		}
		if index+1 >= len(args) {
			return false
		}
		index++
	}
	return false
}
