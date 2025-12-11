package constants

// API Endpoints (used only in cloud mode)
const (
	// Events endpoint
	EVENTS_URL = "https://api.catops.app/api/cli/events"

	// OpenTelemetry Protocol (OTLP) endpoints
	// Metrics are now sent via OTLP instead of REST API
	OTLP_ENDPOINT   = "api.catops.app"  // OTLP HTTP endpoint host (SDK adds /api/v1/metrics)
	OTLP_PATH       = "/api/v1/metrics" // Custom path for CatOps OTLP receiver
	OTLP_LOGS_PATH  = "/api/v1/logs"    // Custom path for CatOps OTLP logs receiver

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

// Default monitoring configuration
const (
	DEFAULT_COLLECTION_INTERVAL = 15 // seconds
)

// File paths
const (
	CONFIG_DIR_NAME = "/.catops"
	PID_FILE        = "/tmp/catops.pid"
	LOG_FILE        = "/tmp/catops.log"
)
