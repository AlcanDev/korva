package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Read and write Korva configuration",
	Long: `Read and write Korva configuration values.

Configuration layers (lower overrides higher):
  Global:  ~/.korva/config.json       applies to all projects
  Local:   ./korva.config.json        current project only

Examples:
  korva config list                   show merged config
  korva config list --global          show global config only
  korva config get project            get a value
  korva config get vault.port         get a nested value
  korva config set project myapp      set in local config
  korva config set vault.port 7438 --global`,
}

var (
	cfgGlobal bool
	cfgLocal  bool
)

func init() {
	for _, sub := range []*cobra.Command{configListCmd, configGetCmd, configSetCmd} {
		sub.Flags().BoolVar(&cfgGlobal, "global", false, "Target global config (~/.korva/config.json)")
		sub.Flags().BoolVar(&cfgLocal, "local", false, "Target local config (./korva.config.json)")
	}
	configCmd.AddCommand(configListCmd, configGetCmd, configSetCmd)
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configuration values",
	RunE:  runConfigList,
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value (dot-separated path)",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigGet,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value (dot-separated path)",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

// ── helpers ───────────────────────────────────────────────────────────────────

func cfgScopeCheck() error {
	if cfgGlobal && cfgLocal {
		return fmt.Errorf("cannot use --global and --local together")
	}
	return nil
}

func globalCfgPath() (string, error) {
	paths, err := config.PlatformPaths()
	if err != nil {
		return "", err
	}
	return paths.ConfigFile, nil
}

func localCfgPath() string {
	return "korva.config.json"
}

func loadCfgMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s: %w", path, err)
	}
	return m, nil
}

func saveCfgMap(path string, m map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// mergeMaps returns a shallow copy of base with overrides applied from top.
func mergeMaps(base, top map[string]any) map[string]any {
	out := make(map[string]any, len(base))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range top {
		if bsub, ok := base[k].(map[string]any); ok {
			if tsub, ok := v.(map[string]any); ok {
				out[k] = mergeMaps(bsub, tsub)
				continue
			}
		}
		out[k] = v
	}
	return out
}

// getNestedValue retrieves a dot-separated path from a nested map.
func getNestedValue(m map[string]any, key string) (any, bool) {
	parts := strings.SplitN(key, ".", 2)
	val, ok := m[parts[0]]
	if !ok {
		return nil, false
	}
	if len(parts) == 1 {
		return val, true
	}
	sub, ok := val.(map[string]any)
	if !ok {
		return nil, false
	}
	return getNestedValue(sub, parts[1])
}

// setNestedValue sets a dot-separated path in a nested map.
func setNestedValue(m map[string]any, key string, value any) {
	parts := strings.SplitN(key, ".", 2)
	if len(parts) == 1 {
		m[key] = value
		return
	}
	sub, _ := m[parts[0]].(map[string]any)
	if sub == nil {
		sub = map[string]any{}
	}
	setNestedValue(sub, parts[1], value)
	m[parts[0]] = sub
}

// parseValue tries to interpret value as bool or number before falling back to string.
func parseValue(s string) any {
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}

func printMapFlat(prefix string, m map[string]any) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		full := k
		if prefix != "" {
			full = prefix + "." + k
		}
		switch v := m[k].(type) {
		case map[string]any:
			printMapFlat(full, v)
		default:
			fmt.Printf("  %-36s %v\n", full, v)
		}
	}
}

// ── commands ──────────────────────────────────────────────────────────────────

func runConfigList(cmd *cobra.Command, _ []string) error {
	if err := cfgScopeCheck(); err != nil {
		return err
	}

	globalPath, err := globalCfgPath()
	if err != nil {
		return err
	}

	switch {
	case cfgGlobal:
		m, err := loadCfgMap(globalPath)
		if err != nil {
			return err
		}
		fmt.Printf("Global config  (%s)\n\n", globalPath)
		printMapFlat("", m)

	case cfgLocal:
		m, err := loadCfgMap(localCfgPath())
		if err != nil {
			return err
		}
		fmt.Printf("Local config  (%s)\n\n", localCfgPath())
		printMapFlat("", m)

	default:
		gm, err := loadCfgMap(globalPath)
		if err != nil {
			return err
		}
		lm, err := loadCfgMap(localCfgPath())
		if err != nil {
			return err
		}
		merged := mergeMaps(gm, lm)
		fmt.Println("Merged config  (local overrides global)")
		fmt.Println()
		printMapFlat("", merged)
		fmt.Println()
		fmt.Printf("  global : %s\n", globalPath)
		fmt.Printf("  local  : %s\n", localCfgPath())
	}
	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	if err := cfgScopeCheck(); err != nil {
		return err
	}

	key := args[0]
	globalPath, err := globalCfgPath()
	if err != nil {
		return err
	}

	lookup := func(path string) (any, bool, error) {
		m, err := loadCfgMap(path)
		if err != nil {
			return nil, false, err
		}
		v, ok := getNestedValue(m, key)
		return v, ok, nil
	}

	switch {
	case cfgGlobal:
		v, ok, err := lookup(globalPath)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("key %q not found in global config", key)
		}
		fmt.Println(formatValue(v))

	case cfgLocal:
		v, ok, err := lookup(localCfgPath())
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("key %q not found in local config", key)
		}
		fmt.Println(formatValue(v))

	default:
		// merged: local wins
		lv, lok, err := lookup(localCfgPath())
		if err != nil {
			return err
		}
		if lok {
			fmt.Println(formatValue(lv))
			return nil
		}
		gv, gok, err := lookup(globalPath)
		if err != nil {
			return err
		}
		if !gok {
			return fmt.Errorf("key %q not found", key)
		}
		fmt.Println(formatValue(gv))
	}
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	if err := cfgScopeCheck(); err != nil {
		return err
	}

	key, rawVal := args[0], args[1]
	value := parseValue(rawVal)

	globalPath, err := globalCfgPath()
	if err != nil {
		return err
	}

	// Default scope for set is local
	target := localCfgPath()
	label := "local"
	if cfgGlobal {
		target = globalPath
		label = "global"
	}

	m, err := loadCfgMap(target)
	if err != nil {
		return err
	}
	setNestedValue(m, key, value)
	if err := saveCfgMap(target, m); err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Set %s = %v  (%s config)", key, value, label))
	return nil
}

func formatValue(v any) string {
	switch val := v.(type) {
	case map[string]any:
		b, _ := json.MarshalIndent(val, "", "  ")
		return string(b)
	case []any:
		b, _ := json.Marshal(val)
		return string(b)
	default:
		return fmt.Sprintf("%v", val)
	}
}
