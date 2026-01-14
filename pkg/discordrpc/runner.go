package discordrpc

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

// RunOptions controls how the custom RPC runner selects a profile.
type RunOptions struct {
	ConfigPath  string
	ProfileName string
	UserName    string
}

// Run loads custom-rpc.json and applies the selected profile.
func Run(appName string, opts RunOptions) error {
	util.SetAppName(appName)

	if err := log.SetupLogger(); err != nil {
		return fmt.Errorf("configure logger: %w", err)
	}
	defer log.GlobalLogger.Sync()

	configPath := strings.TrimSpace(opts.ConfigPath)
	if configPath == "" {
		configPath = util.GetCustomRPCFilePath()
	}

	if err := files.EnsureCustomRPCFileAtPath(configPath); err != nil {
		return fmt.Errorf("ensure custom rpc config: %w", err)
	}

	cfg, err := files.LoadCustomRPCFileFromPath(configPath)
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = &files.CustomRPCConfig{}
	}

	profile, source, err := selectProfile(cfg, opts.ProfileName, opts.UserName)
	if err != nil {
		return err
	}
	if profile == nil {
		return fmt.Errorf("no custom rpc profile selected")
	}
	if strings.TrimSpace(profile.ApplicationID) == "" {
		return fmt.Errorf("profile %q missing application_id", profile.Name)
	}

	var rpc rpcClient
	if err := rpc.Login(profile.ApplicationID); err != nil {
		return fmt.Errorf("discord rpc login failed: %w", err)
	}
	defer rpc.Logout()

	log.ApplicationLogger().Info("Custom RPC connected", "profile", profile.Name, "source", source, "config", configPath)

	startedAt := time.Now()
	if err := updateActivity(&rpc, profile, startedAt, time.Now()); err != nil {
		return err
	}

	interval := resolveUpdateInterval(profile)
	if interval <= 0 {
		util.WaitForInterrupt()
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			if err := updateActivity(&rpc, profile, startedAt, now); err != nil {
				log.ErrorLoggerRaw().Error("Failed to update custom RPC activity", "err", err)
			}
		}
	}
}

func updateActivity(rpc *rpcClient, profile *files.CustomRPCProfile, startedAt, now time.Time) error {
	activity, err := buildActivityPayload(profile, startedAt, now)
	if err != nil {
		return err
	}
	if err := rpc.SetActivity(activity); err != nil {
		return fmt.Errorf("set activity: %w", err)
	}
	return nil
}

func selectProfile(cfg *files.CustomRPCConfig, profileName, userName string) (*files.CustomRPCProfile, string, error) {
	name := strings.TrimSpace(profileName)
	source := "profile"

	if name == "" && strings.TrimSpace(userName) != "" {
		if cfg.UserProfiles == nil {
			return nil, "", fmt.Errorf("user_profiles not configured in custom-rpc.json")
		}
		mapped, ok := cfg.UserProfiles[userName]
		if !ok {
			return nil, "", fmt.Errorf("no profile mapped for user %q", userName)
		}
		name = mapped
		source = "user_profiles"
	}

	if name == "" && strings.TrimSpace(cfg.DefaultProfile) != "" {
		name = cfg.DefaultProfile
		source = "default_profile"
	}

	if name == "" && len(cfg.Profiles) == 1 {
		name = cfg.Profiles[0].Name
		source = "only_profile"
	}

	if name == "" {
		return nil, "", fmt.Errorf("no profile selected; set -profile or default_profile in custom-rpc.json")
	}

	for i := range cfg.Profiles {
		if strings.EqualFold(cfg.Profiles[i].Name, name) {
			if cfg.Profiles[i].Disabled {
				return nil, "", fmt.Errorf("profile %q is disabled", cfg.Profiles[i].Name)
			}
			return &cfg.Profiles[i], source, nil
		}
	}

	return nil, "", fmt.Errorf("profile %q not found (available: %s)", name, strings.Join(listProfileNames(cfg), ", "))
}

func listProfileNames(cfg *files.CustomRPCConfig) []string {
	out := make([]string, 0, len(cfg.Profiles))
	for _, p := range cfg.Profiles {
		if strings.TrimSpace(p.Name) != "" {
			out = append(out, p.Name)
		}
	}
	return out
}

func resolveUpdateInterval(profile *files.CustomRPCProfile) time.Duration {
	if profile.UpdateIntervalSeconds > 0 {
		return time.Duration(profile.UpdateIntervalSeconds) * time.Second
	}
	mode := strings.ToLower(strings.TrimSpace(profile.Timestamp.Mode))
	switch mode {
	case "since_last_update", "last_update":
		return 15 * time.Second
	case "local_time":
		return time.Minute
	default:
		return 0
	}
}

func buildActivityPayload(profile *files.CustomRPCProfile, startedAt, now time.Time) (*rpcActivity, error) {
	activity := &rpcActivity{
		Details: strings.TrimSpace(profile.Details),
		State:   strings.TrimSpace(profile.State),
		URL:     strings.TrimSpace(profile.URL),
	}

	rawType := strings.TrimSpace(profile.Type)
	if rawType != "" {
		activityType, ok := parseActivityType(rawType)
		if !ok {
			return nil, fmt.Errorf("unknown activity type %q", profile.Type)
		}
		activity.Type = &activityType
	}

	if assets := buildAssets(profile); assets != nil {
		activity.Assets = assets
	}
	if party := buildParty(profile); party != nil {
		activity.Party = party
	}
	if timestamps, err := buildTimestamps(profile, startedAt, now); err != nil {
		return nil, err
	} else if timestamps != nil {
		activity.Timestamps = timestamps
	}
	if buttons := buildButtons(profile); len(buttons) > 0 {
		activity.Buttons = buttons
	}

	return activity, nil
}

func parseActivityType(raw string) (int, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "playing":
		return 0, true
	case "streaming":
		return 1, true
	case "listening":
		return 2, true
	case "watching":
		return 3, true
	case "competing":
		return 5, true
	default:
		if n, err := strconv.Atoi(value); err == nil {
			return n, true
		}
		return 0, false
	}
}

func buildAssets(profile *files.CustomRPCProfile) *rpcAssets {
	assets := profile.Assets
	if assets.LargeImageKey == "" && assets.LargeText == "" && assets.SmallImageKey == "" && assets.SmallText == "" {
		return nil
	}
	return &rpcAssets{
		LargeImage: assets.LargeImageKey,
		LargeText:  assets.LargeText,
		SmallImage: assets.SmallImageKey,
		SmallText:  assets.SmallText,
	}
}

func buildParty(profile *files.CustomRPCProfile) *rpcParty {
	party := profile.Party
	if party.ID == "" && party.Current <= 0 && party.Max <= 0 {
		return nil
	}
	current := party.Current
	max := party.Max
	if current < 0 {
		current = 0
	}
	if max < 0 {
		max = 0
	}
	return &rpcParty{
		ID:   party.ID,
		Size: [2]int{current, max},
	}
}

func buildTimestamps(profile *files.CustomRPCProfile, startedAt, now time.Time) (*rpcTimestamps, error) {
	mode := strings.ToLower(strings.TrimSpace(profile.Timestamp.Mode))
	switch mode {
	case "", "none":
		return nil, nil
	case "since_start":
		start := toUnixMillis(startedAt)
		return &rpcTimestamps{Start: &start}, nil
	case "since_last_update", "last_update", "local_time":
		start := toUnixMillis(now)
		return &rpcTimestamps{Start: &start}, nil
	case "custom":
		start, end, err := parseCustomTimestamps(profile.Timestamp)
		if err != nil {
			return nil, err
		}
		if start == nil && end == nil {
			return nil, nil
		}
		return &rpcTimestamps{Start: start, End: end}, nil
	default:
		return nil, fmt.Errorf("unknown timestamp mode %q", profile.Timestamp.Mode)
	}
}

func parseCustomTimestamps(cfg files.RPCTimestampConfig) (*uint64, *uint64, error) {
	var start *uint64
	var end *uint64

	if cfg.StartUnix != 0 {
		v := toUnixMillis(time.Unix(cfg.StartUnix, 0))
		start = &v
	} else if strings.TrimSpace(cfg.Start) != "" {
		t, err := parseTime(cfg.Start)
		if err != nil {
			return nil, nil, err
		}
		v := toUnixMillis(t)
		start = &v
	}

	if cfg.EndUnix != 0 {
		v := toUnixMillis(time.Unix(cfg.EndUnix, 0))
		end = &v
	} else if strings.TrimSpace(cfg.End) != "" {
		t, err := parseTime(cfg.End)
		if err != nil {
			return nil, nil, err
		}
		v := toUnixMillis(t)
		end = &v
	}

	return start, end, nil
}

func parseTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if layout == time.RFC3339 {
			if t, err := time.Parse(layout, value); err == nil {
				return t, nil
			}
			continue
		}
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time %q; use RFC3339 or YYYY-MM-DD HH:MM:SS", value)
}

func buildButtons(profile *files.CustomRPCProfile) []rpcButton {
	if len(profile.Buttons) == 0 {
		return nil
	}
	buttons := make([]rpcButton, 0, 2)
	for _, b := range profile.Buttons {
		label := strings.TrimSpace(b.Label)
		url := strings.TrimSpace(b.URL)
		if label == "" || url == "" {
			continue
		}
		buttons = append(buttons, rpcButton{Label: label, URL: url})
		if len(buttons) == 2 {
			break
		}
	}
	return buttons
}

func toUnixMillis(t time.Time) uint64 {
	return uint64(t.UnixNano() / int64(time.Millisecond))
}
