package constants

// API Endpoints (used only in cloud mode)
const (
	// Analytics and data endpoints
	ANALYTICS_URL = "https://api.moniq.sh/api/data/alerts"
	EVENTS_URL    = "https://api.moniq.sh/api/data/events"

	// Server management endpoints
	SERVERS_URL   = "https://api.moniq.sh/api/servers/change-owner"
	INSTALL_URL   = "https://api.moniq.sh/api/downloads/install"
	UNINSTALL_URL = "https://api.moniq.sh/api/servers/uninstall"

	// Version and update endpoints
	VERSIONS_URL = "https://api.moniq.sh/api/versions/check"

	// Download endpoints
	GET_MONIQ_URL = "https://get.moniq.sh"
)

// Application constants
const (
	USER_AGENT = "Moniq-CLI/1.0.0"
)

// External services
const (
	TELEGRAM_API_URL = "https://api.telegram.org/bot%s/sendMessage"
	MONIQ_WEBSITE    = "https://moniq.sh"
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
	CONFIG_DIR_NAME = "/.moniq"
	PID_FILE        = "/tmp/moniq.pid"
	LOG_FILE        = "/tmp/moniq.log"
)
