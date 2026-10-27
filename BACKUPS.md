# Storage, backups and restores

All application settings, links and click history live in `sho-rt/links.db` under Go's `os.UserConfigDir()`:

- Linux: `$XDG_CONFIG_HOME/sho-rt/links.db`, or `~/.config/sho-rt/links.db`.
- macOS: `~/Library/Application Support/sho-rt/links.db`.
- Windows: `%AppData%/sho-rt/links.db`.

Missing directories and the database are created automatically. The database location is not editable. These paths are OS-aware; they do not imply that native-service integration has been tested on every OS.

No config.json is created or required. A legacy config.json is imported if SQLite has no settings, then renamed to config.json.migrated as an inactive backup. Existing SQLite settings take precedence. If a legacy configuration points to a different database, startup reports an error rather than silently abandoning it: stop all processes, restore a consistent backup at the fixed location, and remove the old JSON file. New installations default to example.com; existing domains are preserved.

For a live backup, use SQLite's backup API/CLI `.backup` command or `VACUUM INTO`; do not copy only the main database file while the application or daemon is running because WAL data may be outstanding. Alternatively stop both processes before copying the database.

To restore, stop the application and daemon, preserve the current database as a safety backup, and restore a consistent snapshot to the OS-specific location above. Do not leave stale WAL/SHM files from an unrelated database alongside the restored snapshot. The application loads its saved domain, IP, port and legacy startup preference. Check these values before starting the server on a different machine, especially an address that exposes it to the network.

## Click statistics and public traffic

Click history is aggregated into hourly UTC buckets, retained for 90 days. Lifetime per-link counts are preserved. The “Last 24 hours” statistic includes the current hour and previous 23 hours (hour-granularity, not an exact rolling window). Old individual click records are migrated automatically; back up before upgrading. Migration does not automatically shrink an existing database file: freed pages are reused by SQLite. Offline SQLite `VACUUM` can reclaim space, but needs spare disk space.

Tracking updates are transactional and best-effort: if recording fails, a successfully resolved link still redirects. Tracking errors are logged at most once per minute per process. This bounds storage growth by links and hours, not request count; it is not a traffic rate limiter. Configure Cloudflare rate-limiting rules separately if your plan supports them. Cloudflared alone does not provide those rules. Keep the listener on loopback when the tunnel is on the same machine.

## Linux daemon setup

Run sho-rt as your normal user. Keep its executable in a permanent location such as `~/.local/bin/sho-rt` before selecting **Install service**. Linux daemon actions use a systemd user service, not a root service, and require a working systemd user session. **Start daemon** starts it immediately; **Enable boot startup** enables the installed unit and lingering so it can start before login and continue after logout. Lingering applies to your whole user manager, not just sho-rt.

Privileged setup asks for confirmation, temporarily releases the TUI terminal, and delegates authentication to sudo. Passwordless/cached sudo works normally; passwords are never collected by the app. Cancellation, missing sudo, or denied permissions are reported. If enabling the unit fails after lingering succeeds, lingering remains enabled and the error is shown; retry after installing the service.

If migrating from a system-wide installation, select **Disable legacy service** first. This stops and disables the old service but does not delete its unit or database. New service installation/start is blocked while the old system service is active or enabled. If the old service used root's configuration directory, restore a consistent database backup into your own configuration directory and ensure you own it before starting the user service. Never run the TUI with sudo to install the new service.

OS service installation, running process state, DNS records, reverse-proxy configuration, and TLS certificates are external to SQLite. Reconfigure or back these up separately. Restoring a database does not install a service or restore DNS/TLS.
