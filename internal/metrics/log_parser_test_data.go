package metrics

// TestLogSamples contains real-world log samples for testing the parser
// Run with: go test -v -run TestLogParserWithSamples
var TestLogSamples = []struct {
	Name     string
	Log      string
	Expected map[string]interface{} // What we expect to extract
}{
	// =====================
	// JSON Logs
	// =====================
	{
		Name: "JSON - Simple",
		Log:  `{"timestamp":"2025-01-15T10:30:00Z","level":"info","message":"User logged in","user_id":"123"}`,
		Expected: map[string]interface{}{
			"level":   "INFO",
			"message": "User logged in",
			"user_id": "123",
		},
	},
	{
		Name: "JSON - FastAPI/Uvicorn",
		Log:  `{"timestamp":"2025-01-15T10:30:00Z","level":"info","message":"Request completed","method":"POST","path":"/api/users","status":201,"duration_ms":45,"request_id":"abc-123"}`,
		Expected: map[string]interface{}{
			"level":       "INFO",
			"http_method": "POST",
			"http_path":   "/api/users",
			"http_status": 201,
			"request_id":  "abc-123",
		},
	},
	{
		Name: "JSON - Node.js Pino",
		Log:  `{"level":30,"time":1705315800000,"pid":1234,"hostname":"server1","msg":"Database connection established","db":"postgres"}`,
		Expected: map[string]interface{}{
			"level":   "INFO", // Pino level 30 = info
			"message": "Database connection established",
		},
	},
	{
		Name: "JSON - Python structlog",
		Log:  `{"event":"Payment processed","level":"info","timestamp":"2025-01-15T10:30:00Z","amount":99.99,"currency":"USD","user_id":"usr_123","trace_id":"abc123"}`,
		Expected: map[string]interface{}{
			"level":    "INFO",
			"message":  "Payment processed", // "event" field
			"trace_id": "abc123",
			"user_id":  "usr_123",
		},
	},
	{
		Name: "JSON - Error with stack",
		Log:  `{"level":"error","message":"Failed to process request","error":"NullPointerException","stack_trace":"at com.app.Service.process(Service.java:42)\n  at com.app.Controller.handle(Controller.java:15)","request_id":"req-456"}`,
		Expected: map[string]interface{}{
			"level":       "ERROR",
			"error_type":  "NullPointerException",
			"stack_trace": "at com.app.Service.process",
			"request_id":  "req-456",
		},
	},
	{
		Name: "JSON - Nested objects",
		Log:  `{"level":"info","msg":"API call","request":{"method":"GET","url":"/api/data","headers":{"Authorization":"Bearer xxx"}},"response":{"status":200,"time_ms":120}}`,
		Expected: map[string]interface{}{
			"level": "INFO",
			// Should extract nested request.method, response.status
		},
	},
	{
		Name: "JSON - Kubernetes pod log",
		Log:  `{"log":"Processing batch job\n","stream":"stdout","time":"2025-01-15T10:30:00.123456789Z","kubernetes":{"pod_name":"worker-abc123","namespace":"production"}}`,
		Expected: map[string]interface{}{
			"message": "Processing batch job",
		},
	},

	// =====================
	// Docker / Logrus format
	// =====================
	{
		Name: "Logrus - Docker daemon",
		Log:  `time="2025-01-15T10:30:00.123456789Z" level=error msg="copy stream failed" error="reading from a closed fifo" stream=stderr`,
		Expected: map[string]interface{}{
			"level":      "ERROR",
			"message":    "copy stream failed",
			"error_type": "reading from a closed fifo",
		},
	},
	{
		Name: "Logrus - With trace context",
		Log:  `time="2025-01-15T10:30:00Z" level=info msg="Request handled" method=POST path=/api/login status=200 duration=45ms traceID=abc123 spanID=def456`,
		Expected: map[string]interface{}{
			"level":       "INFO",
			"http_method": "POST",
			"http_path":   "/api/login",
			"trace_id":    "abc123",
			"span_id":     "def456",
		},
	},

	// =====================
	// Apache / Nginx Common Log Format
	// =====================
	{
		Name: "Apache - Common Log Format",
		Log:  `192.168.1.100 - john [15/Jan/2025:10:30:00 +0000] "GET /index.html HTTP/1.1" 200 2326`,
		Expected: map[string]interface{}{
			"source_ip":   "192.168.1.100",
			"http_method": "GET",
			"http_path":   "/index.html",
			"http_status": 200,
		},
	},
	{
		Name: "Nginx - Combined Log Format",
		Log:  `192.168.1.100 - - [15/Jan/2025:10:30:00 +0000] "POST /api/users HTTP/1.1" 201 1234 "https://example.com" "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"`,
		Expected: map[string]interface{}{
			"source_ip":   "192.168.1.100",
			"http_method": "POST",
			"http_path":   "/api/users",
			"http_status": 201,
			"user_agent":  "Mozilla/5.0",
		},
	},
	{
		Name: "Nginx - Error log",
		Log:  `2025/01/15 10:30:00 [error] 1234#5678: *9999 open() "/var/www/missing.html" failed (2: No such file or directory), client: 192.168.1.100, server: example.com, request: "GET /missing.html HTTP/1.1"`,
		Expected: map[string]interface{}{
			"level":       "ERROR",
			"source_ip":   "192.168.1.100",
			"http_method": "GET",
			"http_path":   "/missing.html",
		},
	},

	// =====================
	// Syslog format
	// =====================
	{
		Name: "Syslog - BSD format",
		Log:  `Jan 15 10:30:00 myserver sshd[1234]: Accepted publickey for user from 192.168.1.50 port 22 ssh2`,
		Expected: map[string]interface{}{
			"source_ip": "192.168.1.50",
			"level":     "INFO",
		},
	},
	{
		Name: "Syslog - RFC5424",
		Log:  `<132>1 2025-01-15T10:30:00.123456Z myserver myapp 1234 ID47 [exampleSDID@32473 iut="3" eventSource="Application"] User authentication failed`,
		Expected: map[string]interface{}{
			"level":   "WARN", // facility 16 * 8 + 4 (warning) = 132
			"message": "User authentication failed",
		},
	},

	// =====================
	// Application specific formats
	// =====================
	{
		Name: "PostgreSQL",
		Log:  `2025-01-15 10:30:00.123 UTC [1234] ERROR:  duplicate key value violates unique constraint "users_email_key"`,
		Expected: map[string]interface{}{
			"level":      "ERROR",
			"error_type": "duplicate key",
		},
	},
	{
		Name: "MySQL",
		Log:  `2025-01-15T10:30:00.123456Z 1234 [ERROR] [MY-010584] [Repl] Slave I/O for channel '': error connecting to master 'repl@master:3306'`,
		Expected: map[string]interface{}{
			"level": "ERROR",
		},
	},
	{
		Name: "Redis",
		Log:  `1234:M 15 Jan 2025 10:30:00.123 * DB loaded from disk: 0.001 seconds`,
		Expected: map[string]interface{}{
			"level": "INFO",
		},
	},
	{
		Name: "MongoDB",
		Log:  `{"t":{"$date":"2025-01-15T10:30:00.123Z"},"s":"E","c":"NETWORK","id":12345,"ctx":"conn1","msg":"Connection refused","attr":{"host":"localhost","port":27017}}`,
		Expected: map[string]interface{}{
			"level":   "ERROR", // s: "E"
			"message": "Connection refused",
		},
	},

	// =====================
	// Python formats
	// =====================
	{
		Name: "Python - Standard logging",
		Log:  `2025-01-15 10:30:00,123 - myapp.module - ERROR - Failed to connect to database: Connection refused`,
		Expected: map[string]interface{}{
			"level":   "ERROR",
			"message": "Failed to connect to database: Connection refused",
		},
	},
	{
		Name: "Python - Django",
		Log:  `[15/Jan/2025 10:30:00] "GET /admin/ HTTP/1.1" 200 12345`,
		Expected: map[string]interface{}{
			"http_method": "GET",
			"http_path":   "/admin/",
			"http_status": 200,
		},
	},
	{
		Name: "Python - Traceback",
		Log: `Traceback (most recent call last):
  File "/app/main.py", line 42, in process
    result = api.call()
  File "/app/api.py", line 15, in call
    raise ConnectionError("Failed to connect")
ConnectionError: Failed to connect`,
		Expected: map[string]interface{}{
			"level":       "ERROR",
			"error_type":  "ConnectionError",
			"stack_trace": "File \"/app/main.py\"",
		},
	},

	// =====================
	// Java / JVM formats
	// =====================
	{
		Name: "Java - Log4j/Logback",
		Log:  `2025-01-15 10:30:00.123 ERROR [main] c.e.a.MyService - Failed to process request: NullPointerException`,
		Expected: map[string]interface{}{
			"level":      "ERROR",
			"error_type": "NullPointerException",
		},
	},
	{
		Name: "Java - Spring Boot",
		Log:  `2025-01-15 10:30:00.123  INFO 1234 --- [nio-8080-exec-1] c.e.app.UserController : User created: id=123, email=user@example.com`,
		Expected: map[string]interface{}{
			"level":   "INFO",
			"user_id": "123",
		},
	},
	{
		Name: "Java - Stack trace",
		Log: `java.lang.NullPointerException: Cannot invoke method on null object
	at com.example.app.Service.process(Service.java:42)
	at com.example.app.Controller.handle(Controller.java:15)
	at sun.reflect.NativeMethodAccessorImpl.invoke0(Native Method)`,
		Expected: map[string]interface{}{
			"level":       "ERROR",
			"error_type":  "NullPointerException",
			"stack_trace": "at com.example.app.Service.process",
		},
	},

	// =====================
	// Go formats
	// =====================
	{
		Name: "Go - Standard log",
		Log:  `2025/01/15 10:30:00 http: Server starting on :8080`,
		Expected: map[string]interface{}{
			"level": "INFO",
		},
	},
	{
		Name: "Go - Zap JSON",
		Log:  `{"level":"error","ts":1705315800.123,"caller":"server/main.go:42","msg":"Request failed","error":"connection refused","method":"GET","path":"/api/health"}`,
		Expected: map[string]interface{}{
			"level":       "ERROR",
			"message":     "Request failed",
			"http_method": "GET",
			"http_path":   "/api/health",
		},
	},
	{
		Name: "Go - Zerolog",
		Log:  `{"level":"info","time":"2025-01-15T10:30:00Z","caller":"main.go:42","message":"Server started","port":8080}`,
		Expected: map[string]interface{}{
			"level":   "INFO",
			"message": "Server started",
		},
	},

	// =====================
	// Node.js formats
	// =====================
	{
		Name: "Node - console.log with prefix",
		Log:  `[2025-01-15T10:30:00.123Z] INFO: Server listening on port 3000`,
		Expected: map[string]interface{}{
			"level": "INFO",
		},
	},
	{
		Name: "Node - Winston",
		Log:  `{"level":"error","message":"Unhandled rejection","stack":"Error: ECONNREFUSED\n    at Socket.emit (node:events:513:28)\n    at emitErrorNT (node:internal/streams/destroy:157:8)","timestamp":"2025-01-15T10:30:00.123Z"}`,
		Expected: map[string]interface{}{
			"level":       "ERROR",
			"error_type":  "ECONNREFUSED",
			"stack_trace": "at Socket.emit",
		},
	},
	{
		Name: "Node - PM2",
		Log:  `PM2        | 2025-01-15T10:30:00: PM2 log: App [myapp:0] starting in -fork mode-`,
		Expected: map[string]interface{}{
			"level": "INFO",
		},
	},

	// =====================
	// Kubernetes / Container formats
	// =====================
	{
		Name: "K8s - Event",
		Log:  `W0115 10:30:00.123456       1 reflector.go:324] k8s.io/client-go/informers/factory.go:134: watch of *v1.Pod ended with: very short watch`,
		Expected: map[string]interface{}{
			"level": "WARN",
		},
	},
	{
		Name: "K8s - kubectl logs",
		Log:  `{"ts":"2025-01-15T10:30:00.123Z","level":"info","logger":"controller.deployment","msg":"Reconciling deployment","namespace":"default","name":"myapp"}`,
		Expected: map[string]interface{}{
			"level":   "INFO",
			"message": "Reconciling deployment",
		},
	},

	// =====================
	// Security / Auth logs
	// =====================
	{
		Name: "Auth - Failed login",
		Log:  `2025-01-15 10:30:00 WARN  [security] - Failed login attempt for user 'admin' from IP 192.168.1.100 - reason: invalid password`,
		Expected: map[string]interface{}{
			"level":     "WARN",
			"source_ip": "192.168.1.100",
			"user_id":   "admin",
		},
	},
	{
		Name: "Auth - JWT validation",
		Log:  `{"level":"warn","message":"JWT validation failed","error":"token expired","user_id":"usr_123","ip":"10.0.0.1","timestamp":"2025-01-15T10:30:00Z"}`,
		Expected: map[string]interface{}{
			"level":      "WARN",
			"error_type": "token expired",
			"user_id":    "usr_123",
			"source_ip":  "10.0.0.1",
		},
	},

	// =====================
	// Uvicorn / Gunicorn (Docker Compose logs)
	// =====================
	{
		Name: "Uvicorn - Access log",
		Log:  `172.19.0.1:35730 - "GET /rates/?currency_from=RUB&currency_to=USDT HTTP/1.0" 200`,
		Expected: map[string]interface{}{
			"source_ip":   "172.19.0.1",
			"http_method": "GET",
			"http_path":   "/rates/?currency_from=RUB&currency_to=USDT",
			"http_status": 200,
			"level":       "INFO",
		},
	},
	{
		Name: "Uvicorn - Access log POST",
		Log:  `10.0.0.5:12345 - "POST /api/v1/orders HTTP/1.1" 201`,
		Expected: map[string]interface{}{
			"source_ip":   "10.0.0.5",
			"http_method": "POST",
			"http_path":   "/api/v1/orders",
			"http_status": 201,
			"level":       "INFO",
		},
	},
	{
		Name: "Uvicorn - Access log error",
		Log:  `192.168.1.50:8080 - "GET /admin/config HTTP/1.1" 500`,
		Expected: map[string]interface{}{
			"source_ip":   "192.168.1.50",
			"http_method": "GET",
			"http_path":   "/admin/config",
			"http_status": 500,
			"level":       "ERROR",
		},
	},
	{
		Name: "Gunicorn - Error log",
		Log:  `[2025-12-22 18:02:09 +0000] [10] [WARNING] Invalid HTTP request received.`,
		Expected: map[string]interface{}{
			"level":   "WARN",
			"message": "Invalid HTTP request received.",
		},
	},
	{
		Name: "Gunicorn - Info log",
		Log:  `[2025-01-15 10:30:00 +0000] [1234] [INFO] Starting gunicorn 20.1.0`,
		Expected: map[string]interface{}{
			"level":   "INFO",
			"message": "Starting gunicorn 20.1.0",
		},
	},
	{
		Name: "Gunicorn - Critical log",
		Log:  `[2025-01-15 10:30:00 +0000] [1] [CRITICAL] Worker failed to boot.`,
		Expected: map[string]interface{}{
			"level":   "FATAL",
			"message": "Worker failed to boot.",
		},
	},
	{
		Name: "Docker prefix - Uvicorn",
		Log:  `easytopay_backend  | 172.19.0.1:35730 - "GET /health HTTP/1.0" 200`,
		Expected: map[string]interface{}{
			"source_ip":   "172.19.0.1",
			"http_method": "GET",
			"http_path":   "/health",
			"http_status": 200,
		},
	},
	{
		Name: "Docker prefix - Gunicorn",
		Log:  `my_app_container  | [2025-01-15 10:30:00 +0000] [10] [ERROR] Connection refused`,
		Expected: map[string]interface{}{
			"level":   "ERROR",
			"message": "Connection refused",
		},
	},

	// =====================
	// Edge cases
	// =====================
	{
		Name: "Multiline - SQL query",
		Log: `2025-01-15 10:30:00 DEBUG [db] - Executing query:
SELECT u.id, u.name, u.email
FROM users u
WHERE u.status = 'active'
ORDER BY u.created_at DESC`,
		Expected: map[string]interface{}{
			"level": "DEBUG",
		},
	},
	{
		Name: "Mixed - JSON in text",
		Log:  `2025-01-15 10:30:00 INFO Received webhook payload: {"event":"user.created","data":{"id":"123","email":"user@example.com"}}`,
		Expected: map[string]interface{}{
			"level": "INFO",
		},
	},
	{
		Name: "Unicode - Non-ASCII",
		Log:  `{"level":"info","message":"Пользователь вошёл в систему","user_id":"123","timestamp":"2025-01-15T10:30:00Z"}`,
		Expected: map[string]interface{}{
			"level":   "INFO",
			"message": "Пользователь вошёл в систему",
			"user_id": "123",
		},
	},
	{
		Name: "High cardinality - UUID",
		Log:  `{"level":"info","message":"Processing request","request_id":"550e8400-e29b-41d4-a716-446655440000","trace_id":"abc123def456","user_id":"usr_789"}`,
		Expected: map[string]interface{}{
			"request_id": "550e8400-e29b-41d4-a716-446655440000",
			"trace_id":   "abc123def456",
			"user_id":    "usr_789",
		},
	},
}
