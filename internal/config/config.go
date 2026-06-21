package config

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/isyuricunha/discord-news-rss-bot/internal/model"
	"github.com/isyuricunha/discord-news-rss-bot/internal/text"
)

const (
	DefaultConfigPath = "/etc/discord-rss-bot/config.env"
	DefaultDBName     = "posted_hashes.db"
	DefaultHealthName = "health.json"
)

type InitialSyncMode string

const (
	InitialSyncSkip     InitialSyncMode = "skip"
	InitialSyncLatest   InitialSyncMode = "latest"
	InitialSyncBackfill InitialSyncMode = "backfill"
)

type Config struct {
	DiscordWebhookURL string
	ConfigPath        string
	DataDir           string
	DBFile            string
	HealthFile        string

	Feeds []model.FeedConfig

	CheckInterval       time.Duration
	PostDelay           time.Duration
	CooldownDelay       time.Duration
	MaxPostLength       int
	MaxContentLength    int
	FeedTimeout         time.Duration
	InitialSyncMode     InitialSyncMode
	InitialSyncMaxPosts int
	MaxEntriesPerFeed   int
	MaxPostsPerCycle    int
	MaxConcurrentFeeds  int
	PostRetentionDays   int
	MaxFeedBytes        int64
	DiscordMaxRetries   int
	LogLevel            string
	LogFormat           string
	HealthMaxAge        time.Duration

	Deprecated []string
}

type LoadOptions struct {
	RequireWebhook bool
	Env            map[string]string
	OS             string
}

func Load(options LoadOptions) (Config, error) {
	env := options.Env
	if env == nil {
		env = environMap(os.Environ())
	}
	osName := options.OS
	if osName == "" {
		osName = runtime.GOOS
	}

	configPath := firstNonEmpty(env["RSS_BOT_CONFIG"], DefaultConfigPath)
	fileValues := map[string]string{}
	if fileExists(configPath) {
		values, err := ParseConfigFile(configPath)
		if err != nil {
			return Config{}, err
		}
		fileValues = values
	}

	get := func(key string) string {
		if value, ok := env[key]; ok {
			return value
		}
		return fileValues[key]
	}

	dataDir := firstNonEmpty(get("RSS_BOT_DATA"), get("DATA_DIR"), defaultDataDir(osName))
	dbFile := firstNonEmpty(get("DB_FILE"), filepath.Join(dataDir, DefaultDBName))
	cfg := Config{
		DiscordWebhookURL:   strings.TrimSpace(get("DISCORD_WEBHOOK_URL")),
		ConfigPath:          configPath,
		DataDir:             dataDir,
		DBFile:              dbFile,
		HealthFile:          filepath.Join(dataDir, DefaultHealthName),
		CheckInterval:       300 * time.Second,
		PostDelay:           3 * time.Second,
		CooldownDelay:       60 * time.Second,
		MaxPostLength:       1900,
		MaxContentLength:    800,
		FeedTimeout:         30 * time.Second,
		InitialSyncMode:     InitialSyncSkip,
		InitialSyncMaxPosts: 1,
		MaxEntriesPerFeed:   20,
		MaxPostsPerCycle:    10,
		MaxConcurrentFeeds:  5,
		PostRetentionDays:   365,
		MaxFeedBytes:        10_485_760,
		DiscordMaxRetries:   5,
		LogLevel:            "info",
		LogFormat:           "text",
	}

	var err error
	if value := get("CHECK_INTERVAL"); value != "" {
		cfg.CheckInterval, err = parseDurationSetting("CHECK_INTERVAL", value, false)
		if err != nil {
			return Config{}, err
		}
	}
	if value := get("POST_DELAY"); value != "" {
		cfg.PostDelay, err = parseDurationSetting("POST_DELAY", value, false)
		if err != nil {
			return Config{}, err
		}
	}
	if value := get("COOLDOWN_DELAY"); value != "" {
		cfg.CooldownDelay, err = parseDurationSetting("COOLDOWN_DELAY", value, false)
		if err != nil {
			return Config{}, err
		}
	}
	if value := get("FEED_TIMEOUT"); value != "" {
		cfg.FeedTimeout, err = parseDurationSetting("FEED_TIMEOUT", value, false)
		if err != nil {
			return Config{}, err
		}
	}
	if value := get("MAX_POST_LENGTH"); value != "" {
		cfg.MaxPostLength, err = parseIntSetting("MAX_POST_LENGTH", value, 1, text.DiscordContentLimit)
		if err != nil {
			return Config{}, err
		}
	}
	if value := get("MAX_CONTENT_LENGTH"); value != "" {
		cfg.MaxContentLength, err = parseIntSetting("MAX_CONTENT_LENGTH", value, 1, text.DiscordContentLimit)
		if err != nil {
			return Config{}, err
		}
	}
	if value := get("INITIAL_SYNC_MODE"); value != "" {
		cfg.InitialSyncMode = InitialSyncMode(strings.ToLower(strings.TrimSpace(value)))
		switch cfg.InitialSyncMode {
		case InitialSyncSkip, InitialSyncLatest, InitialSyncBackfill:
		default:
			return Config{}, fmt.Errorf("INITIAL_SYNC_MODE must be one of skip, latest, backfill")
		}
	}
	if value := get("INITIAL_SYNC_MAX_POSTS"); value != "" {
		cfg.InitialSyncMaxPosts, err = parseIntSetting("INITIAL_SYNC_MAX_POSTS", value, 1, 10_000)
		if err != nil {
			return Config{}, err
		}
	}
	if value := get("MAX_ENTRIES_PER_FEED"); value != "" {
		cfg.MaxEntriesPerFeed, err = parseIntSetting("MAX_ENTRIES_PER_FEED", value, 1, 10_000)
		if err != nil {
			return Config{}, err
		}
	}
	if value := get("MAX_POSTS_PER_CYCLE"); value != "" {
		cfg.MaxPostsPerCycle, err = parseIntSetting("MAX_POSTS_PER_CYCLE", value, 1, 10_000)
		if err != nil {
			return Config{}, err
		}
	}
	if value := get("MAX_CONCURRENT_FEEDS"); value != "" {
		cfg.MaxConcurrentFeeds, err = parseIntSetting("MAX_CONCURRENT_FEEDS", value, 1, 1_000)
		if err != nil {
			return Config{}, err
		}
	}
	if value := get("POST_RETENTION_DAYS"); value != "" {
		cfg.PostRetentionDays, err = parseIntSetting("POST_RETENTION_DAYS", value, 0, 100_000)
		if err != nil {
			return Config{}, err
		}
	}
	if value := get("MAX_FEED_BYTES"); value != "" {
		parsed, parseErr := parseIntSetting("MAX_FEED_BYTES", value, 1, 1_073_741_824)
		if parseErr != nil {
			return Config{}, parseErr
		}
		cfg.MaxFeedBytes = int64(parsed)
	}
	if value := get("DISCORD_MAX_RETRIES"); value != "" {
		cfg.DiscordMaxRetries, err = parseIntSetting("DISCORD_MAX_RETRIES", value, 0, 100)
		if err != nil {
			return Config{}, err
		}
	}
	if value := get("LOG_LEVEL"); value != "" {
		cfg.LogLevel = strings.ToLower(strings.TrimSpace(value))
		if !isAllowed(cfg.LogLevel, "debug", "info", "warn", "error") {
			return Config{}, fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error")
		}
	}
	if value := get("LOG_FORMAT"); value != "" {
		cfg.LogFormat = strings.ToLower(strings.TrimSpace(value))
		if !isAllowed(cfg.LogFormat, "text", "json") {
			return Config{}, fmt.Errorf("LOG_FORMAT must be text or json")
		}
	}
	if value := get("HEALTH_MAX_AGE"); value != "" {
		cfg.HealthMaxAge, err = parseDurationSetting("HEALTH_MAX_AGE", value, true)
		if err != nil {
			return Config{}, err
		}
	}
	if cfg.HealthMaxAge == 0 {
		cfg.HealthMaxAge = cfg.CheckInterval*3 + cfg.FeedTimeout + 30*time.Second
	}

	feeds, err := parseFeeds(get, env, fileValues)
	if err != nil {
		return Config{}, err
	}
	cfg.Feeds = feeds

	if options.RequireWebhook && strings.TrimSpace(cfg.DiscordWebhookURL) == "" {
		return Config{}, errors.New("DISCORD_WEBHOOK_URL is required")
	}

	for _, legacy := range []string{"RSS_BOT_LOGS", "LOG_FILE", "RSS_BOT_PID"} {
		if get(legacy) != "" {
			cfg.Deprecated = append(cfg.Deprecated, legacy)
		}
	}

	return cfg, nil
}

func ParseConfigFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected KEY=VALUE", path, lineNumber)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return nil, fmt.Errorf("%s:%d: empty key", path, lineNumber)
		}
		if strings.ContainsAny(key, " \t") {
			return nil, fmt.Errorf("%s:%d: key %q contains whitespace", path, lineNumber, key)
		}
		unquoted, err := unquoteValue(value)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
		}
		values[key] = unquoted
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func Redact(value string) string {
	return text.RedactSecret(value)
}

func parseFeeds(get func(string) string, env, fileValues map[string]string) ([]model.FeedConfig, error) {
	if universal := get("RSS_FEEDS"); strings.TrimSpace(universal) != "" {
		return parseFeedList("📢 Universal Feeds", universal, nil)
	}

	categoryValues := map[string]string{}
	for key, value := range fileValues {
		if strings.HasPrefix(key, "RSS_FEEDS_") && key != "RSS_FEEDS" {
			categoryValues[key] = value
		}
	}
	for key, value := range env {
		if strings.HasPrefix(key, "RSS_FEEDS_") && key != "RSS_FEEDS" {
			categoryValues[key] = value
		}
	}

	keys := make([]string, 0, len(categoryValues))
	for key := range categoryValues {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var feeds []model.FeedConfig
	seen := map[string]struct{}{}
	for _, key := range keys {
		categoryKey := strings.TrimPrefix(key, "RSS_FEEDS_")
		categoryName := categoryTitle(categoryKey)
		parsed, err := parseFeedList(categoryName, categoryValues[key], seen)
		if err != nil {
			return nil, err
		}
		feeds = append(feeds, parsed...)
	}
	if len(feeds) > 0 {
		return feeds, nil
	}

	for _, group := range defaultFeedGroups() {
		parsed, err := parseFeedList(group.category, strings.Join(group.urls, ","), seen)
		if err != nil {
			return nil, err
		}
		feeds = append(feeds, parsed...)
	}
	return feeds, nil
}

func parseFeedList(category string, raw string, seen map[string]struct{}) ([]model.FeedConfig, error) {
	if seen == nil {
		seen = map[string]struct{}{}
	}
	var feeds []model.FeedConfig
	for _, part := range strings.Split(raw, ",") {
		feedURL := strings.TrimSpace(part)
		if feedURL == "" {
			continue
		}
		if err := validateHTTPURL(feedURL); err != nil {
			return nil, err
		}
		key := model.NormalizeFeedURL(feedURL)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		feeds = append(feeds, model.FeedConfig{
			URL:      feedURL,
			Source:   SourceName(feedURL),
			Category: category,
			Emoji:    categoryEmoji(category),
		})
	}
	return feeds, nil
}

func SourceName(feedURL string) string {
	domainMapping := map[string]string{
		"g1.globo.com":               "G1",
		"rss.uol.com.br":             "UOL",
		"band.uol.com.br":            "Band",
		"cnnbrasil.com.br":           "CNN Brasil",
		"feeds.folha.uol.com.br":     "Folha",
		"gazetadopovo.com.br":        "Gazeta do Povo",
		"jovempan.com.br":            "Jovem Pan",
		"diariodopoder.com.br":       "Diario do Poder",
		"pragmatismopolitico.com.br": "Pragmatismo Politico",
		"conexaopolitica.com.br":     "Conexao Politica",
		"poder360.com.br":            "Poder 360",
		"crusoe.uol.com.br":          "Crusoe",
		"veja.abril.com.br":          "Veja",
		"metropoles.com":             "Metropoles",
		"oantagonista.com":           "O Antagonista",
		"terra.com.br":               "Terra",
		"canaltech.com.br":           "Canaltech",
		"olhardigital.com.br":        "Olhar Digital",
		"tecnoblog.net":              "Tecnoblog",
		"meiobit.com":                "Meio Bit",
		"showmetech.com.br":          "ShowMeTech",
		"tecmundo.com.br":            "TecMundo",
		"adrenaline.com.br":          "Adrenaline",
		"hardware.com.br":            "Hardware.com.br",
		"tudocelular.com":            "Tudo Celular",
		"oficinadanet.com.br":        "Oficina da Net",
	}
	parsed, err := url.Parse(feedURL)
	if err != nil {
		return "Source"
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	for domain, source := range domainMapping {
		if strings.Contains(host, domain) {
			return source
		}
	}
	parts := strings.Split(host, ".")
	if len(parts) == 0 || parts[0] == "" {
		return "Source"
	}
	return strings.Title(parts[0])
}

func defaultDataDir(osName string) string {
	if osName == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "DiscordRSSBot")
		}
		return filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local", "DiscordRSSBot")
	}
	return "/var/lib/discord-rss-bot"
}

func categoryTitle(categoryKey string) string {
	name := strings.ReplaceAll(strings.ToLower(categoryKey), "_", " ")
	words := strings.Fields(name)
	for i, word := range words {
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	titled := strings.Join(words, " ")
	return categoryEmoji(titled) + " " + titled
}

func categoryEmoji(category string) string {
	lower := strings.ToLower(category)
	switch {
	case strings.Contains(lower, "news"), strings.Contains(lower, "noticias"), strings.Contains(lower, "general"):
		return "📰"
	case strings.Contains(lower, "tech"), strings.Contains(lower, "technology"), strings.Contains(lower, "tecnologia"):
		return "💻"
	case strings.Contains(lower, "politics"), strings.Contains(lower, "politica"), strings.Contains(lower, "conservative"):
		return "🏛️"
	case strings.Contains(lower, "sports"), strings.Contains(lower, "esportes"):
		return "⚽"
	case strings.Contains(lower, "business"), strings.Contains(lower, "economia"), strings.Contains(lower, "finance"):
		return "💼"
	default:
		return "📢"
	}
}

func validateHTTPURL(raw string) error {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return fmt.Errorf("invalid feed URL %q: %w", text.SanitizeURL(raw), err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("invalid feed URL %q: scheme must be http or https", text.SanitizeURL(raw))
	}
	if parsed.Host == "" {
		return fmt.Errorf("invalid feed URL %q: host is required", text.SanitizeURL(raw))
	}
	return nil
}

func parseDurationSetting(name, value string, allowZero bool) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 || (seconds == 0 && !allowZero) {
			return 0, fmt.Errorf("%s must be greater than zero", name)
		}
		return time.Duration(seconds) * time.Second, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be seconds or a Go duration: %w", name, err)
	}
	if duration < 0 || (duration == 0 && !allowZero) {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return duration, nil
}

func parseIntSetting(name, value string, minValue, maxValue int) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	if parsed < minValue {
		if minValue == 0 {
			return 0, fmt.Errorf("%s must be zero or greater", name)
		}
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	if parsed > maxValue {
		return 0, fmt.Errorf("%s must be less than or equal to %d", name, maxValue)
	}
	return parsed, nil
}

func unquoteValue(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(value, `"`) || strings.HasPrefix(value, `'`) {
		quote := value[:1]
		if !strings.HasSuffix(value, quote) || len(value) == 1 {
			return "", fmt.Errorf("unterminated quoted value")
		}
		return value[1 : len(value)-1], nil
	}
	if strings.Contains(value, "`") {
		return "", fmt.Errorf("backtick shell syntax is not supported")
	}
	return value, nil
}

func isAllowed(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func environMap(values []string) map[string]string {
	env := map[string]string{}
	for _, entry := range values {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			env[key] = value
		}
	}
	return env
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type defaultFeedGroup struct {
	category string
	urls     []string
}

func defaultFeedGroups() []defaultFeedGroup {
	return []defaultFeedGroup{
		{
			category: "📰 General News",
			urls: []string{
				"https://g1.globo.com/dynamo/rss2.xml",
				"https://rss.uol.com.br/feed/noticias.xml",
				"https://www.band.uol.com.br/rss/noticias.xml",
				"https://www.cnnbrasil.com.br/rss/",
				"https://feeds.folha.uol.com.br/folha/rss02.xml",
			},
		},
		{
			category: "🏛️ Politics & Conservative",
			urls: []string{
				"https://www.gazetadopovo.com.br/rss/brasil.xml",
				"https://jovempan.com.br/rss.xml",
				"https://www.diariodopoder.com.br/feed/",
				"https://www.pragmatismopolitico.com.br/feed/",
				"https://conexaopolitica.com.br/feed/",
				"https://www.poder360.com.br/feed/",
				"https://crusoe.uol.com.br/rss/",
				"https://veja.abril.com.br/rss/",
				"https://www.metropoles.com/rss.xml",
				"https://www.oantagonista.com/rss/",
				"https://www.terra.com.br/rss/politica/",
			},
		},
		{
			category: "💻 Technology",
			urls: []string{
				"https://canaltech.com.br/rss/",
				"https://olhardigital.com.br/feed/",
				"https://tecnoblog.net/feed/",
				"https://meiobit.com/feed/",
				"https://www.showmetech.com.br/feed/",
				"https://www.tecmundo.com.br/rss",
				"https://www.adrenaline.com.br/rss/",
				"https://www.hardware.com.br/rss/",
				"https://www.tudocelular.com/rss/",
				"https://www.oficinadanet.com.br/rss",
			},
		},
	}
}
