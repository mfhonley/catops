package constants

// API Endpoints (used only in cloud mode)
const (
	// Analytics and data endpoints
	ANALYTICS_URL = "https://api.catops.io/api/cli/alerts"
	EVENTS_URL    = "https://api.catops.io/api/cli/events"
	METRICS_URL   = "https://api.catops.io/api/cli/metrics"

	// Server management endpoints
	SERVERS_URL   = "https://api.catops.io/api/cli/servers/change-owner"
	INSTALL_URL   = "https://api.catops.io/api/cli/install"
	UNINSTALL_URL = "https://api.catops.io/api/cli/uninstall"

	// Version and update endpoints
	VERSIONS_URL = "https://api.catops.io/api/versions/check"

	// Download endpoints
	GET_CATOPS_URL = "https://get.catops.io"
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
	CATOPS_WEBSITE    = "https://catops.io"
)

// Operation modes
const (
	MODE_LOCAL = "local" // No backend dependency
	MODE_CLOUD = "cloud" // Requires backend integration
)

// Default thresholds
const (
	DEFAULT_CPU_THRESHOLD    = 50.0
	DEFAULT_MEMORY_THRESHOLD = 50.0
	DEFAULT_DISK_THRESHOLD   = 50.0
)

// File paths
const (
	CONFIG_DIR_NAME = "/.catops"
	PID_FILE        = "/tmp/catops.pid"
	LOG_FILE        = "/tmp/catops.log"
)
