package constants

// API Endpoints (used only in cloud mode)
const (
	// Analytics and data endpoints
	ANALYTICS_URL = "https://api.moniq.sh/api/data/alerts"
	EVENTS_URL    = "https://api.moniq.sh/api/data/events"

	// Server management endpoints
	SERVERS_URL = "https://api.moniq.sh/api/servers/change-owner"
	INSTALL_URL = "https://api.moniq.sh/api/downloads/install"

	// Version and update endpoints
	VERSIONS_URL = "https://api.moniq.sh/api/versions/check"
	UPDATES_URL  = "https://api.moniq.sh/api/versions/new"

	// Download endpoints
	GET_MONIQ_URL = "https://get.moniq.sh"
)

// Application constants
const (
	APP_NAME    = "Moniq CLI"
	APP_VERSION = "0.1.6" // Current version from version.txt
	USER_AGENT  = "Moniq-CLI/0.1.6"
)

// Operation modes
const (
	MODE_LOCAL = "local" // No backend dependency
	MODE_CLOUD = "cloud" // Requires backend integration
)

// Default thresholds
const (
	DEFAULT_CPU_THRESHOLD    = 70.0
	DEFAULT_MEMORY_THRESHOLD = 80.0
	DEFAULT_DISK_THRESHOLD   = 90.0
	DEFAULT_CHECK_INTERVAL   = 30 // seconds
)

// File paths
const (
	CONFIG_DIR_NAME  = "/.moniq"
	CONFIG_FILE_NAME = "config.yaml"
	PID_FILE         = "/tmp/moniq.pid"
	LOG_FILE         = "/tmp/moniq.log"
)
