package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alcandev/korva/internal/config"
	"github.com/alcandev/korva/internal/hive"
	"github.com/alcandev/korva/internal/identity"
	"github.com/alcandev/korva/internal/license"
	"github.com/alcandev/korva/internal/privacy/cloud"
	"github.com/alcandev/korva/internal/version"
	"github.com/alcandev/korva/vault/internal/api"
	"github.com/alcandev/korva/vault/internal/email"
	"github.com/alcandev/korva/vault/internal/mcp"
	"github.com/alcandev/korva/vault/internal/store"
	"github.com/alcandev/korva/vault/internal/tui"
	"github.com/alcandev/korva/vault/internal/ui"
)

func main() {
	mode   := flag.String("mode", "both", "Server mode: mcp | http | both | tui")
	port   := flag.Int("port", 7437, "HTTP server port")
	dbPath := flag.String("db", "", "SQLite database path (default: ~/.korva/vault/observations.db)")
	flag.Parse()

	// Root context — cancelled on SIGINT/SIGTERM so all goroutines can clean up.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	paths, err := config.PlatformPaths()
	if err != nil {
		log.Fatalf("Cannot determine platform paths: %v", err)
	}

	if err := paths.EnsureAll(); err != nil {
		log.Fatalf("Cannot create korva directories: %v", err)
	}

	if *dbPath == "" {
		*dbPath = paths.VaultDB()
	}

	s, err := store.New(*dbPath, nil)
	if err != nil {
		log.Fatalf("Cannot open vault store: %v", err)
	}
	defer s.Close()

	// License — nil on community tier, safe for all feature checks.
	lic, _ := license.Load(paths.LicenseFile)

	// Email — reads KORVA_EMAIL_API_KEY and KORVA_EMAIL_FROM from environment.
	// Returns a silent noop when not configured.
	mailer := email.NewFromEnv()
	if mailer.Configured() {
		log.Printf("Email delivery enabled (provider: resend)")
	}

	// Load platform config (falls back to defaults when file is missing).
	cfg, _ := config.Load(paths.ConfigFile)

	// Boot Teams-tier services (heartbeat + license expiry warning).
	bootLicense(ctx, lic, cfg, paths)

	// Boot the Hive worker if enabled.
	hiveResult := bootHive(ctx, s, cfg, paths)

	// Boot retention worker if configured.
	bootRetention(ctx, s, cfg)

	// Resolve activation URL — env var overrides config.
	activationURL := cfg.License.ActivationURL
	if ep := os.Getenv("KORVA_LICENSING_ENDPOINT"); ep != "" {
		activationURL = strings.TrimRight(ep, "/") + "/v1/activate"
	}

	// InstallID is needed for license activate — tolerate missing file.
	installID, _ := identity.LoadInstallID(paths.InstallID)

	routerCfg := api.RouterConfig{
		AdminKeyPath:     paths.AdminKey,
		License:          lic,
		LicensePath:      paths.LicenseFile,
		LicenseStatePath: paths.LicenseStateFile,
		ActivationURL:    activationURL,
		InstallID:        installID,
		Mailer:           mailer,
		HiveClient:       hiveResult.Client,
		HiveWorker:       hiveResult.Worker,
		HiveOutbox:       hiveResult.Outbox,
		HiveFilter:       hiveResult.Filter,
		WebhookURL:       cfg.Vault.WebhookURL,
	}

	switch *mode {
	case "mcp":
		runMCP(s, hiveResult.Client)
	case "http":
		runHTTP(ctx, s, routerCfg, *port)
	case "tui":
		runTUI(s)
	case "both":
		go runHTTP(ctx, s, routerCfg, *port)
		runMCP(s, hiveResult.Client)
		// MCP exited (stdin closed) — trigger HTTP shutdown.
		stop()
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode %q — use: mcp | http | both | tui\n", *mode)
		os.Exit(1)
	}
}

// bootLicense wires the license heartbeat and logs expiry warnings.
// Safe to call with a nil license (community tier — no-op).
func bootLicense(ctx context.Context, lic *license.License, cfg config.KorvaConfig, paths *config.Paths) {
	if lic == nil {
		log.Printf("Korva Vault %s — community tier", version.Version)
		return
	}

	// Expiry warning — show 14 days before the key expires.
	daysLeft := time.Until(lic.ExpiresAt).Hours() / 24
	if daysLeft < 0 {
		log.Printf("⚠  License EXPIRED on %s — run 'korva license activate <key>' to renew",
			lic.ExpiresAt.Format("2006-01-02"))
	} else if daysLeft < 14 {
		log.Printf("⚠  License expires in %.0f day(s) (%s) — run 'korva license activate <key>' to renew",
			daysLeft, lic.ExpiresAt.Format("2006-01-02"))
	} else {
		log.Printf("Korva Vault %s — %s tier (expires %s)",
			version.Version, lic.Tier, lic.ExpiresAt.Format("2006-01-02"))
	}

	// Wire the 24-h heartbeat to keep the Teams license current.
	installID, err := identity.LoadInstallID(paths.InstallID)
	if err != nil {
		log.Printf("license heartbeat: cannot load install.id (%v) — heartbeat disabled", err)
		return
	}

	// Derive the heartbeat URL from the activation URL.
	heartbeatURL := deriveHeartbeatURL(cfg.License.ActivationURL)
	if ep := os.Getenv("KORVA_LICENSING_ENDPOINT"); ep != "" {
		heartbeatURL = strings.TrimRight(ep, "/") + "/v1/heartbeat"
	}

	license.RunHeartbeat(ctx, heartbeatURL, installID, paths.LicenseStateFile, lic)
	log.Printf("license: heartbeat enabled (url=%s)", heartbeatURL)
}

// deriveHeartbeatURL converts the activation URL to the heartbeat URL.
// "https://licensing.korva.dev/v1/activate" → "https://licensing.korva.dev/v1/heartbeat"
func deriveHeartbeatURL(activationURL string) string {
	if idx := strings.LastIndex(activationURL, "/v1/"); idx != -1 {
		return activationURL[:idx] + "/v1/heartbeat"
	}
	return "https://licensing.korva.dev/v1/heartbeat"
}

// bootHiveResult groups all Hive boot outputs.
type bootHiveResult struct {
	Client *hive.Client
	Worker *hive.Worker
	Outbox *hive.Outbox
	Filter *cloud.Filter
}

// bootHive starts the Hive worker goroutine when the config enables it.
// It wires the outbox into the store so every Save call enqueues an observation.
// Returns a result with all Hive components, or zero values when Hive is disabled.
func bootHive(ctx context.Context, s *store.Store, cfg config.KorvaConfig, paths *config.Paths) bootHiveResult {
	if !cfg.Hive.Enabled {
		return bootHiveResult{}
	}
	// Allow the environment to override the endpoint (useful for dev/testing).
	endpoint := cfg.Hive.Endpoint
	if ep := os.Getenv("KORVA_HIVE_ENDPOINT"); ep != "" {
		endpoint = ep
	}

	installID, idErr := identity.LoadInstallID(paths.InstallID)
	if idErr != nil {
		log.Printf("hive: cannot load install.id (%v) — Hive sync disabled", idErr)
		return bootHiveResult{}
	}
	hiveKey, hkErr := identity.LoadHiveKey(paths.HiveKey)
	if hkErr != nil {
		log.Printf("hive: cannot load hive.key (%v) — Hive sync disabled", hkErr)
		return bootHiveResult{}
	}

	outbox := hive.NewOutbox(s.DB())
	s.AttachHive(outbox)

	cloudFilter := cloud.New(cfg.Hive.AllowedTypes, installID)
	client := hive.NewClient(endpoint, hiveKey)

	interval := time.Duration(cfg.Hive.IntervalMin) * time.Minute
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	worker := hive.NewWorker(outbox, client, cloudFilter, installID, interval)
	go worker.Run(ctx)

	log.Printf("hive: worker started (endpoint=%s, interval=%v)", endpoint, interval)
	return bootHiveResult{Client: client, Worker: worker, Outbox: outbox, Filter: cloudFilter}
}

// bootRetention starts a daily goroutine that auto-purges observations older
// than cfg.Vault.RetentionDays. Does nothing when RetentionDays is 0 (default).
func bootRetention(ctx context.Context, s *store.Store, cfg config.KorvaConfig) {
	days := cfg.Vault.RetentionDays
	if days <= 0 {
		return
	}
	log.Printf("retention: auto-purge enabled (%d days)", days)
	go func() {
		// First run after 1 minute to allow the store to warm up.
		select {
		case <-time.After(1 * time.Minute):
		case <-ctx.Done():
			return
		}
		for {
			cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
			n, err := s.Purge(store.PurgeOptions{Before: cutoff})
			if err != nil {
				log.Printf("retention: purge error: %v", err)
			} else if n > 0 {
				log.Printf("retention: purged %d observation(s) older than %d day(s)", n, days)
			}
			select {
			case <-time.After(24 * time.Hour):
			case <-ctx.Done():
				return
			}
		}
	}()
}

// hiveSearchAdapter bridges hive.Client to mcp.CloudSearcher.
// It converts hive.SearchResult into mcp.CloudHit so the vault mcp package
// never needs to import the internal/hive package (avoiding circular deps).
type hiveSearchAdapter struct{ c *hive.Client }

func (a *hiveSearchAdapter) Search(ctx context.Context, query string, limit int) ([]mcp.CloudHit, error) {
	results, err := a.c.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	hits := make([]mcp.CloudHit, len(results))
	for i, r := range results {
		hits[i] = mcp.CloudHit{
			ID:      r.ID,
			Type:    r.Type,
			Title:   r.Title,
			Content: r.Content,
			Source:  r.Source,
		}
	}
	return hits, nil
}

func runMCP(s *store.Store, hiveClient *hive.Client) {
	server := mcp.New(s)
	if hiveClient != nil {
		server.WithCloudSearch(&hiveSearchAdapter{c: hiveClient})
		log.Printf("MCP: hybrid search enabled (local + hive)")
	}
	if err := server.Run(); err != nil {
		log.Fatalf("MCP server error: %v", err)
	}
}

func runTUI(s *store.Store) {
	m := tui.New(s)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("TUI error: %v", err)
	}
}

// runHTTP mounts the vault REST API and the Beacon SPA on a single HTTP server
// with graceful shutdown on context cancellation.
//
// Route layout:
//
//	/vault-api/*   →  vault REST/admin API  (Beacon's API prefix, stripped before dispatch)
//	/healthz       →  vault health check
//	/api/v1/*      →  vault REST API (direct access from CLI / curl)
//	/admin/*       →  vault admin API (direct access)
//	/*             →  Beacon SPA (static files + SPA fallback to index.html)
func runHTTP(ctx context.Context, s *store.Store, cfg api.RouterConfig, port int) {
	vaultAPI := api.Router(s, cfg)

	// beaconDev is the URL to redirect to when the UI is not embedded.
	// In development, Vite runs on :5173 and proxies /vault-api → :7437 itself.
	beaconDev := "http://localhost:5173"

	spaHandler := ui.Handler(beaconDev)

	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path

		// Beacon calls all API endpoints via /vault-api/* — strip the prefix
		// so the internal router sees the canonical paths it expects.
		if strings.HasPrefix(p, "/vault-api") {
			r2 := r.Clone(r.Context())
			r2.URL.Path = strings.TrimPrefix(p, "/vault-api")
			if r2.URL.Path == "" {
				r2.URL.Path = "/"
			}
			// Rewrite raw path too, if set.
			if r.URL.RawPath != "" {
				r2.URL.RawPath = strings.TrimPrefix(r.URL.RawPath, "/vault-api")
			}
			vaultAPI.ServeHTTP(w, r2)
			return
		}

		// Direct vault API paths (curl, CLI, MCP HTTP client)
		if p == "/healthz" ||
			strings.HasPrefix(p, "/api/") ||
			strings.HasPrefix(p, "/admin/") {
			vaultAPI.ServeHTTP(w, r)
			return
		}

		// Everything else → Beacon SPA
		spaHandler.ServeHTTP(w, r)
	})

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      baseHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start the server in a goroutine so we can wait on ctx.Done().
	go func() {
		if ui.DistFS != nil {
			log.Printf("Korva Vault listening on http://%s (Beacon UI embedded)", addr)
		} else {
			log.Printf("Korva Vault listening on http://%s  →  Beacon: %s", addr, beaconDev)
		}
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Block until the root context is cancelled (SIGINT/SIGTERM or MCP exit).
	<-ctx.Done()

	log.Printf("Korva Vault shutting down…")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}
}
