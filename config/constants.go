package constants

// API Endpoints (used only in cloud mode)
const (
	// Analytics and data endpoints
	ALERTS_PROCESS_URL   = "https://api.catops.app/api/cli/alerts/process" // Phase 2: New spike-based alerts
	ALERTS_HEARTBEAT_URL = "https://api.catops.app/api/cli/alerts"         // Phase 2: Base URL for heartbeat (/{fingerprint}/heartbeat)
	ALERTS_RESOLVE_URL   = "https://api.catops.app/api/cli/alerts/resolve" // Phase 2: Alert resolution
	EVENTS_URL           = "https://api.catops.app/api/cli/events"
	METRICS_URL          = "https://api.catops.app/api/cli/metrics"
	PROCESSES_URL        = "https://api.catops.app/api/cli/processes"
	NETWORK_METRICS_URL  = "https://api.catops.app/api/cli/network"  // Network observability metrics
	SERVICES_URL         = "https://api.catops.app/api/cli/services" // Service detection metrics

	// Server management endpoints
	SERVERS_URL   = "https://api.catops.app/api/cli/servers/change-owner"
	INSTALL_URL   = "https://api.catops.app/api/cli/install"
	UNINSTALL_URL = "https://api.catops.app/api/cli/uninstall"

	// Version and update endpoints
	VERSIONS_BASE_URL = "https://api.catops.app/api/versions"
	VERSIONS_URL      = "https://api.catops.app/api/versions/check"

	// Download endpoints
	GET_CATOPS_URL = "https://get.catops.app"
)

// HTTP headers required by new backend
const (
	HEADER_USER_AGENT = "CatOps-CLI/1.0.0"
	HEADER_PLATFORM   = "X-Platform"
	HEADER_VERSION    = "X-Version"
)

// External services
const (
	TELEGRAM_API_URL = "https://api.telegram.org/bot%s/sendMessage"
	CATOPS_WEBSITE   = "https://catops.app"
	CATOPS_API_URL   = "https://api.catops.app"
)

// Operation modes
const (
	MODE_LOCAL = "local" // No backend dependency
	MODE_CLOUD = "cloud" // Requires backend integration
)

// Default thresholds (production-ready values to prevent alert spam)
const (
	DEFAULT_CPU_THRESHOLD    = 85.0
	DEFAULT_MEMORY_THRESHOLD = 90.0
	DEFAULT_DISK_THRESHOLD   = 95.0
)

// Default monitoring configuration
const (
	DEFAULT_COLLECTION_INTERVAL       = 15   // seconds
	DEFAULT_BUFFER_SIZE               = 20   // data points (5 minutes at 15s interval)
	DEFAULT_SUDDEN_SPIKE_THRESHOLD    = 30.0 // percent change
	DEFAULT_GRADUAL_RISE_THRESHOLD    = 15.0 // percent change over window
	DEFAULT_ANOMALY_THRESHOLD         = 4.0  // standard deviations
	DEFAULT_ALERT_DEDUPLICATION       = true // enable deduplication
	DEFAULT_ALERT_RENOTIFY_INTERVAL   = 120  // minutes (2 hours)
	DEFAULT_ALERT_RESOLUTION_TIMEOUT  = 5    // minutes
	DETECTION_WINDOW_MINUTES          = 5    // fixed window for gradual rise and anomaly detection
)

// File paths
const (
	CONFIG_DIR_NAME = "/.catops"
	PID_FILE        = "/tmp/catops.pid"
	LOG_FILE        = "/tmp/catops.log"
)
