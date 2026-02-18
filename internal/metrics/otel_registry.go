// Package metrics provides OpenTelemetry metrics registration for CatOps CLI.
package metrics

import (
	"context"
	"encoding/json"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// =============================================================================
// OTel Metrics Registration
// =============================================================================

func registerAllMetrics() error {
	// System Summary Metrics
	if err := registerSystemSummaryMetrics(); err != nil {
		return err
	}

	// Per-core CPU Metrics
	if err := registerCPUCoreMetrics(); err != nil {
		return err
	}

	// Memory Metrics
	if err := registerMemoryMetrics(); err != nil {
		return err
	}

	// Per-mount Disk Metrics
	if err := registerDiskMetrics(); err != nil {
		return err
	}

	// Per-interface Network Metrics
	if err := registerNetworkMetrics(); err != nil {
		return err
	}

	// Process Metrics
	if err := registerProcessMetrics(); err != nil {
		return err
	}

	// Service Metrics
	if err := registerServiceMetrics(); err != nil {
		return err
	}

	// Container Metrics
	if err := registerContainerMetrics(); err != nil {
		return err
	}

	// Log Metrics
	if err := registerLogMetrics(); err != nil {
		return err
	}

	return nil
}

func registerSystemSummaryMetrics() error {
	// catops.system.* - Main dashboard metrics
	_, err := meter.Float64ObservableGauge(
		"catops.system.cpu",
		metric.WithDescription("System CPU metrics"),
		metric.WithUnit("%"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			m := GetCachedMetrics()
			if m == nil || m.Summary == nil {
				return nil
			}
			s := m.Summary
			o.Observe(s.CPUUsage, metric.WithAttributes(attribute.String("type", "usage")))
			o.Observe(s.CPUUser, metric.WithAttributes(attribute.String("type", "user")))
			o.Observe(s.CPUSystem, metric.WithAttributes(attribute.String("type", "system")))
			o.Observe(s.CPUIdle, metric.WithAttributes(attribute.String("type", "idle")))
			o.Observe(s.CPUIOWait, metric.WithAttributes(attribute.String("type", "iowait")))
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Float64ObservableGauge(
		"catops.system.load",
		metric.WithDescription("System load averages"),
		metric.WithUnit("1"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			m := GetCachedMetrics()
			if m == nil || m.Summary == nil {
				return nil
			}
			s := m.Summary
			o.Observe(s.Load1m, metric.WithAttributes(attribute.String("period", "1m")))
			o.Observe(s.Load5m, metric.WithAttributes(attribute.String("period", "5m")))
			o.Observe(s.Load15m, metric.WithAttributes(attribute.String("period", "15m")))
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.system.memory",
		metric.WithDescription("System memory in bytes"),
		metric.WithUnit("By"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			m := GetCachedMetrics()
			if m == nil || m.Summary == nil {
				return nil
			}
			s := m.Summary
			o.Observe(int64(s.MemoryTotal), metric.WithAttributes(attribute.String("type", "total")))
			o.Observe(int64(s.MemoryUsed), metric.WithAttributes(attribute.String("type", "used")))
			o.Observe(int64(s.MemoryAvailable), metric.WithAttributes(attribute.String("type", "available")))
			o.Observe(int64(s.MemoryCached), metric.WithAttributes(attribute.String("type", "cached")))
			o.Observe(int64(s.MemoryBuffers), metric.WithAttributes(attribute.String("type", "buffers")))
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Float64ObservableGauge(
		"catops.system.memory.usage",
		metric.WithDescription("System memory usage percent"),
		metric.WithUnit("%"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			m := GetCachedMetrics()
			if m == nil || m.Summary == nil {
				return nil
			}
			o.Observe(m.Summary.MemoryUsage)
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.system.swap",
		metric.WithDescription("System swap in bytes"),
		metric.WithUnit("By"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			m := GetCachedMetrics()
			if m == nil || m.Summary == nil {
				return nil
			}
			s := m.Summary
			o.Observe(int64(s.SwapTotal), metric.WithAttributes(attribute.String("type", "total")))
			o.Observe(int64(s.SwapUsed), metric.WithAttributes(attribute.String("type", "used")))
			o.Observe(int64(s.SwapFree), metric.WithAttributes(attribute.String("type", "free")))
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.system.disk",
		metric.WithDescription("System disk aggregated in bytes"),
		metric.WithUnit("By"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			m := GetCachedMetrics()
			if m == nil || m.Summary == nil {
				return nil
			}
			s := m.Summary
			o.Observe(int64(s.DiskTotal), metric.WithAttributes(attribute.String("type", "total")))
			o.Observe(int64(s.DiskUsed), metric.WithAttributes(attribute.String("type", "used")))
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Float64ObservableGauge(
		"catops.system.disk.usage",
		metric.WithDescription("System disk usage percent"),
		metric.WithUnit("%"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			m := GetCachedMetrics()
			if m == nil || m.Summary == nil {
				return nil
			}
			o.Observe(m.Summary.DiskUsage)
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.system.disk.iops",
		metric.WithDescription("System disk IOPS"),
		metric.WithUnit("{operations}/s"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			m := GetCachedMetrics()
			if m == nil || m.Summary == nil {
				return nil
			}
			s := m.Summary
			o.Observe(int64(s.DiskIOPSRead), metric.WithAttributes(attribute.String("direction", "read")))
			o.Observe(int64(s.DiskIOPSWrite), metric.WithAttributes(attribute.String("direction", "write")))
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.system.network",
		metric.WithDescription("System network aggregated bytes"),
		metric.WithUnit("By"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			m := GetCachedMetrics()
			if m == nil || m.Summary == nil {
				return nil
			}
			s := m.Summary
			o.Observe(int64(s.NetBytesRecv), metric.WithAttributes(attribute.String("direction", "recv")))
			o.Observe(int64(s.NetBytesSent), metric.WithAttributes(attribute.String("direction", "sent")))
			return nil
		}),
	)
	if err != nil {
		return err
	}

	// Network connection states
	_, err = meter.Int64ObservableGauge(
		"catops.system.network.connections",
		metric.WithDescription("Network connection states"),
		metric.WithUnit("{connections}"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			m := GetCachedMetrics()
			if m == nil || m.Summary == nil {
				return nil
			}
			s := m.Summary
			o.Observe(int64(s.NetConnections), metric.WithAttributes(attribute.String("state", "total")))
			o.Observe(int64(s.NetConnectionsEstablished), metric.WithAttributes(attribute.String("state", "established")))
			o.Observe(int64(s.NetConnectionsTimeWait), metric.WithAttributes(attribute.String("state", "time_wait")))
			o.Observe(int64(s.NetConnectionsCloseWait), metric.WithAttributes(attribute.String("state", "close_wait")))
			o.Observe(int64(s.NetConnectionsListen), metric.WithAttributes(attribute.String("state", "listen")))
			o.Observe(int64(s.NetConnectionsSynSent), metric.WithAttributes(attribute.String("state", "syn_sent")))
			o.Observe(int64(s.NetConnectionsSynRecv), metric.WithAttributes(attribute.String("state", "syn_recv")))
			o.Observe(int64(s.NetConnectionsFinWait1), metric.WithAttributes(attribute.String("state", "fin_wait1")))
			o.Observe(int64(s.NetConnectionsFinWait2), metric.WithAttributes(attribute.String("state", "fin_wait2")))
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.system.processes",
		metric.WithDescription("System process counts"),
		metric.WithUnit("{processes}"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			m := GetCachedMetrics()
			if m == nil || m.Summary == nil {
				return nil
			}
			s := m.Summary
			o.Observe(int64(s.ProcessesTotal), metric.WithAttributes(attribute.String("state", "total")))
			o.Observe(int64(s.ProcessesRunning), metric.WithAttributes(attribute.String("state", "running")))
			o.Observe(int64(s.ProcessesSleeping), metric.WithAttributes(attribute.String("state", "sleeping")))
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.system.uptime",
		metric.WithDescription("System uptime in seconds"),
		metric.WithUnit("s"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			m := GetCachedMetrics()
			if m == nil || m.Summary == nil {
				return nil
			}
			o.Observe(int64(m.Summary.UptimeSeconds))
			return nil
		}),
	)
	return err
}

func registerCPUCoreMetrics() error {
	_, err := meter.Float64ObservableGauge(
		"catops.cpu.core",
		metric.WithDescription("Per-core CPU metrics"),
		metric.WithUnit("%"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			m := GetCachedMetrics()
			if m == nil {
				return nil
			}

			for _, core := range m.CPUCores {
				attrs := []attribute.KeyValue{
					attribute.Int("core_id", core.CoreID),
				}
				o.Observe(core.Usage, metric.WithAttributes(append(attrs, attribute.String("type", "usage"))...))
				o.Observe(core.User, metric.WithAttributes(append(attrs, attribute.String("type", "user"))...))
				o.Observe(core.System, metric.WithAttributes(append(attrs, attribute.String("type", "system"))...))
				o.Observe(core.Idle, metric.WithAttributes(append(attrs, attribute.String("type", "idle"))...))
				o.Observe(core.IOWait, metric.WithAttributes(append(attrs, attribute.String("type", "iowait"))...))
				o.Observe(core.IRQ, metric.WithAttributes(append(attrs, attribute.String("type", "irq"))...))
				o.Observe(core.SoftIRQ, metric.WithAttributes(append(attrs, attribute.String("type", "softirq"))...))
				o.Observe(core.Steal, metric.WithAttributes(append(attrs, attribute.String("type", "steal"))...))
				o.Observe(core.Nice, metric.WithAttributes(append(attrs, attribute.String("type", "nice"))...))
			}
			return nil
		}),
	)
	return err
}

func registerMemoryMetrics() error {
	_, err := meter.Int64ObservableGauge(
		"catops.memory.detailed",
		metric.WithDescription("Detailed memory metrics in bytes"),
		metric.WithUnit("By"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			m := GetCachedMetrics()
			if m == nil || m.Memory == nil {
				return nil
			}
			mem := m.Memory
			o.Observe(int64(mem.Total), metric.WithAttributes(attribute.String("type", "total")))
			o.Observe(int64(mem.Used), metric.WithAttributes(attribute.String("type", "used")))
			o.Observe(int64(mem.Free), metric.WithAttributes(attribute.String("type", "free")))
			o.Observe(int64(mem.Available), metric.WithAttributes(attribute.String("type", "available")))
			o.Observe(int64(mem.Cached), metric.WithAttributes(attribute.String("type", "cached")))
			o.Observe(int64(mem.Buffers), metric.WithAttributes(attribute.String("type", "buffers")))
			o.Observe(int64(mem.Shared), metric.WithAttributes(attribute.String("type", "shared")))
			o.Observe(int64(mem.Slab), metric.WithAttributes(attribute.String("type", "slab")))
			o.Observe(int64(mem.SwapTotal), metric.WithAttributes(attribute.String("type", "swap_total")))
			o.Observe(int64(mem.SwapUsed), metric.WithAttributes(attribute.String("type", "swap_used")))
			o.Observe(int64(mem.SwapFree), metric.WithAttributes(attribute.String("type", "swap_free")))
			o.Observe(int64(mem.SwapCached), metric.WithAttributes(attribute.String("type", "swap_cached")))
			return nil
		}),
	)
	return err
}

func registerDiskMetrics() error {
	_, err := meter.Float64ObservableGauge(
		"catops.disk.mount",
		metric.WithDescription("Per-mount disk metrics"),
		metric.WithUnit("%"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			m := GetCachedMetrics()
			if m == nil {
				return nil
			}

			for _, d := range m.Disks {
				attrs := []attribute.KeyValue{
					attribute.String("device", d.Device),
					attribute.String("mount_point", d.MountPoint),
					attribute.String("fs_type", d.FSType),
				}
				o.Observe(d.UsagePercent, metric.WithAttributes(append(attrs, attribute.String("metric", "usage_percent"))...))
				o.Observe(d.InodesPercent, metric.WithAttributes(append(attrs, attribute.String("metric", "inodes_percent"))...))
			}
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.disk.mount.bytes",
		metric.WithDescription("Per-mount disk bytes"),
		metric.WithUnit("By"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			m := GetCachedMetrics()
			if m == nil {
				return nil
			}

			for _, d := range m.Disks {
				attrs := []attribute.KeyValue{
					attribute.String("device", d.Device),
					attribute.String("mount_point", d.MountPoint),
					attribute.String("fs_type", d.FSType),
				}
				o.Observe(int64(d.Total), metric.WithAttributes(append(attrs, attribute.String("type", "total"))...))
				o.Observe(int64(d.Used), metric.WithAttributes(append(attrs, attribute.String("type", "used"))...))
				o.Observe(int64(d.Free), metric.WithAttributes(append(attrs, attribute.String("type", "free"))...))
			}
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.disk.mount.iops",
		metric.WithDescription("Per-mount disk IOPS"),
		metric.WithUnit("{operations}/s"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			m := GetCachedMetrics()
			if m == nil {
				return nil
			}

			for _, d := range m.Disks {
				attrs := []attribute.KeyValue{
					attribute.String("device", d.Device),
					attribute.String("mount_point", d.MountPoint),
				}
				o.Observe(int64(d.IOPSRead), metric.WithAttributes(append(attrs, attribute.String("direction", "read"))...))
				o.Observe(int64(d.IOPSWrite), metric.WithAttributes(append(attrs, attribute.String("direction", "write"))...))
			}
			return nil
		}),
	)
	return err
}

func registerNetworkMetrics() error {
	_, err := meter.Int64ObservableGauge(
		"catops.network.interface.bytes",
		metric.WithDescription("Per-interface network bytes"),
		metric.WithUnit("By"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			m := GetCachedMetrics()
			if m == nil {
				return nil
			}

			for _, n := range m.Networks {
				attrs := []attribute.KeyValue{
					attribute.String("interface", n.Interface),
				}
				o.Observe(int64(n.BytesRecv), metric.WithAttributes(append(attrs, attribute.String("direction", "recv"))...))
				o.Observe(int64(n.BytesSent), metric.WithAttributes(append(attrs, attribute.String("direction", "sent"))...))
			}
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.network.interface.packets",
		metric.WithDescription("Per-interface network packets"),
		metric.WithUnit("{packets}"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			m := GetCachedMetrics()
			if m == nil {
				return nil
			}

			for _, n := range m.Networks {
				attrs := []attribute.KeyValue{
					attribute.String("interface", n.Interface),
				}
				o.Observe(int64(n.PacketsRecv), metric.WithAttributes(append(attrs, attribute.String("direction", "recv"))...))
				o.Observe(int64(n.PacketsSent), metric.WithAttributes(append(attrs, attribute.String("direction", "sent"))...))
			}
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.network.interface.errors",
		metric.WithDescription("Per-interface network errors"),
		metric.WithUnit("{errors}"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			m := GetCachedMetrics()
			if m == nil {
				return nil
			}

			for _, n := range m.Networks {
				attrs := []attribute.KeyValue{
					attribute.String("interface", n.Interface),
				}
				o.Observe(int64(n.ErrorsIn), metric.WithAttributes(append(attrs, attribute.String("direction", "in"))...))
				o.Observe(int64(n.ErrorsOut), metric.WithAttributes(append(attrs, attribute.String("direction", "out"))...))
				o.Observe(int64(n.DropsIn), metric.WithAttributes(append(attrs, attribute.String("type", "drops_in"))...))
				o.Observe(int64(n.DropsOut), metric.WithAttributes(append(attrs, attribute.String("type", "drops_out"))...))
			}
			return nil
		}),
	)
	if err != nil {
		return err
	}

	_, err = meter.Int64ObservableGauge(
		"catops.network.interface.rate",
		metric.WithDescription("Per-interface network rate bytes/sec"),
		metric.WithUnit("By/s"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			m := GetCachedMetrics()
			if m == nil {
				return nil
			}

			for _, n := range m.Networks {
				attrs := []attribute.KeyValue{
					attribute.String("interface", n.Interface),
				}
				o.Observe(int64(n.BytesRecvRate), metric.WithAttributes(append(attrs, attribute.String("direction", "recv"))...))
				o.Observe(int64(n.BytesSentRate), metric.WithAttributes(append(attrs, attribute.String("direction", "sent"))...))
			}
			return nil
		}),
	)
	return err
}

func registerProcessMetrics() error {
	_, err := meter.Float64ObservableGauge(
		"catops.process",
		metric.WithDescription("Process CPU/Memory usage"),
		metric.WithUnit("%"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			m := GetCachedMetrics()
			if m == nil {
				return nil
			}

			limit := 20
			if len(m.Processes) < limit {
				limit = len(m.Processes)
			}

			for i := 0; i < limit; i++ {
				p := m.Processes[i]
				attrs := []attribute.KeyValue{
					attribute.Int("pid", p.PID),
					attribute.Int("ppid", p.PPID),
					attribute.String("name", p.Name),
					attribute.String("command", truncateString(p.Command, 200)),
					attribute.String("exe", p.Exe),
					attribute.String("user", p.User),
					attribute.Int("uid", int(p.UID)),
					attribute.Int("gid", int(p.GID)),
					attribute.String("status", p.Status),
					attribute.Int("num_threads", int(p.NumThreads)),
					attribute.Int("num_fds", int(p.NumFDs)),
					attribute.Int64("memory_rss", int64(p.MemoryRSS)),
					attribute.Int64("memory_vms", int64(p.MemoryVMS)),
					attribute.Int64("memory_shared", int64(p.MemoryShared)),
					attribute.Int64("io_read_bytes", int64(p.IOReadBytes)),
					attribute.Int64("io_write_bytes", int64(p.IOWriteBytes)),
					attribute.Int64("create_time", p.CreateTime),
					attribute.Float64("cpu_time_user", p.CPUTimeUser),
					attribute.Float64("cpu_time_system", p.CPUTimeSystem),
					attribute.Int("nice", int(p.Nice)),
					attribute.Int("priority", int(p.Priority)),
				}
				o.Observe(p.CPUPercent, metric.WithAttributes(append(attrs, attribute.String("metric", "cpu"))...))
				o.Observe(p.MemoryPercent, metric.WithAttributes(append(attrs, attribute.String("metric", "memory"))...))
			}
			return nil
		}),
	)
	return err
}

func registerServiceMetrics() error {
	_, err := meter.Float64ObservableGauge(
		"catops.service",
		metric.WithDescription("Service CPU/Memory usage"),
		metric.WithUnit("%"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			m := GetCachedMetrics()
			if m == nil {
				return nil
			}

			for _, s := range m.Services {
				portsJSON, _ := json.Marshal(s.Ports)
				pidsJSON, _ := json.Marshal(s.PIDs)
				logsJSON, _ := json.Marshal(s.RecentLogs)
				attrs := []attribute.KeyValue{
					attribute.String("service_type", string(s.ServiceType)),
					attribute.String("service_name", s.ServiceName),
					attribute.Int("pid", s.PID),
					attribute.String("pids", string(pidsJSON)),
					attribute.String("ports", string(portsJSON)),
					attribute.String("protocol", s.Protocol),
					attribute.String("bind_address", s.BindAddress),
					attribute.String("version", s.Version),
					attribute.String("config_path", s.ConfigPath),
					attribute.String("status", s.Status),
					attribute.Bool("is_container", s.IsContainer),
					attribute.String("container_id", s.ContainerID),
					attribute.String("container_name", s.ContainerName),
					attribute.String("health_status", s.HealthStatus),
					attribute.Int("connections_active", int(s.ConnectionsActive)),
					attribute.Int64("memory_bytes", int64(s.MemoryBytes)),
					attribute.String("recent_logs", string(logsJSON)),
					attribute.String("log_source", s.LogSource),
				}
				o.Observe(s.CPUPercent, metric.WithAttributes(append(attrs, attribute.String("metric", "cpu"))...))
				o.Observe(s.MemoryPercent, metric.WithAttributes(append(attrs, attribute.String("metric", "memory"))...))
			}
			return nil
		}),
	)
	return err
}

func registerContainerMetrics() error {
	_, err := meter.Float64ObservableGauge(
		"catops.container",
		metric.WithDescription("Container CPU/Memory usage"),
		metric.WithUnit("%"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			m := GetCachedMetrics()
			if m == nil {
				return nil
			}

			for _, c := range m.Containers {
				var exitCode int64 = -1
				if c.ExitCode != nil {
					exitCode = int64(*c.ExitCode)
				}
				logsJSON, _ := json.Marshal(c.RecentLogs)
				attrs := []attribute.KeyValue{
					attribute.String("container_id", c.ContainerID),
					attribute.String("container_name", c.ContainerName),
					attribute.String("image_name", c.ImageName),
					attribute.String("image_tag", c.ImageTag),
					attribute.String("runtime", c.Runtime),
					attribute.String("status", c.Status),
					attribute.String("health", c.Health),
					attribute.Int64("started_at", c.StartedAt),
					attribute.Int64("exit_code", exitCode),
					attribute.Float64("cpu_system_percent", c.CPUSystemPercent),
					attribute.Int64("memory_usage", int64(c.MemoryUsage)),
					attribute.Int64("memory_limit", int64(c.MemoryLimit)),
					attribute.Int64("net_rx_bytes", int64(c.NetRxBytes)),
					attribute.Int64("net_tx_bytes", int64(c.NetTxBytes)),
					attribute.Int64("block_read_bytes", int64(c.BlockReadBytes)),
					attribute.Int64("block_write_bytes", int64(c.BlockWriteBytes)),
					attribute.Int("pids_current", int(c.PIDsCurrent)),
					attribute.Int("pids_limit", int(c.PIDsLimit)),
					attribute.String("ports", c.Ports),
					attribute.String("labels", c.Labels),
					attribute.String("recent_logs", string(logsJSON)),
				}
				o.Observe(c.CPUPercent, metric.WithAttributes(append(attrs, attribute.String("metric", "cpu"))...))
				o.Observe(c.MemoryPercent, metric.WithAttributes(append(attrs, attribute.String("metric", "memory"))...))
			}
			return nil
		}),
	)
	return err
}

func registerLogMetrics() error {
	// catops.log - Log entries from containers and services
	// Value is always 1 (presence indicator); uniqueness guaranteed by message_hash attribute.
	// Using message_hash instead of sequential idx prevents OTel SDK from deduplicating
	// observations that share the same attribute set (which happened when idx=0 was used
	// for the first line of every container).
	_, err := meter.Int64ObservableGauge(
		"catops.log",
		metric.WithDescription("Log entries from containers and services"),
		metric.WithUnit("{entries}"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			m := GetCachedMetrics()
			if m == nil {
				return nil
			}

			// Logs from containers
			for _, c := range m.Containers {
				for _, logLine := range c.RecentLogs {
					msgHash := hashLogMessage(c.ContainerID + logLine)
					level := detectLogLevel(logLine)
					attrs := []attribute.KeyValue{
						attribute.String("source", "docker"),
						attribute.String("source_path", c.ContainerName),
						attribute.String("level", level),
						attribute.String("message", truncateString(logLine, 500)),
						attribute.String("service", c.ContainerName),
						attribute.String("container_id", c.ContainerID),
						attribute.String("message_hash", msgHash),
						attribute.Int("pid", 0),
					}
					o.Observe(1, metric.WithAttributes(attrs...))
				}
			}

			// Logs from services (PM2, non-docker)
			for _, s := range m.Services {
				if s.IsContainer || len(s.RecentLogs) == 0 {
					continue
				}
				for _, logLine := range s.RecentLogs {
					msgHash := hashLogMessage(s.ServiceName + logLine)
					level := detectLogLevel(logLine)
					attrs := []attribute.KeyValue{
						attribute.String("source", s.LogSource),
						attribute.String("source_path", s.ServiceName),
						attribute.String("level", level),
						attribute.String("message", truncateString(logLine, 500)),
						attribute.String("service", s.ServiceName),
						attribute.String("container_id", s.ContainerID),
						attribute.String("message_hash", msgHash),
						attribute.Int("pid", s.PID),
					}
					o.Observe(1, metric.WithAttributes(attrs...))
				}
			}
			return nil
		}),
	)
	return err
}

// hashLogMessage returns a short hex hash for use as a unique OTel attribute
func hashLogMessage(s string) string {
	h := uint64(14695981039346656037)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	const hex = "0123456789abcdef"
	var buf [16]byte
	for i := 15; i >= 0; i-- {
		buf[i] = hex[h&0xf]
		h >>= 4
	}
	return string(buf[:])
}

// detectLogLevel detects log level from log line content
func detectLogLevel(line string) string {
	lineLower := strings.ToLower(line)
	switch {
	case strings.Contains(lineLower, "fatal") || strings.Contains(lineLower, "critical"):
		return "fatal"
	case strings.Contains(lineLower, "error") || strings.Contains(lineLower, "err"):
		return "error"
	case strings.Contains(lineLower, "warn"):
		return "warn"
	case strings.Contains(lineLower, "debug"):
		return "debug"
	default:
		return "info"
	}
}
