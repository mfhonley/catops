# CatOps Troubleshooting Guide

Comprehensive guide for diagnosing and fixing common CatOps issues.

---

## Table of Contents

- [Standalone Server Issues](#standalone-server-issues)
- [Kubernetes Issues](#kubernetes-issues)
- [Telegram Bot Issues](#telegram-bot-issues)
- [Cloud Mode Issues](#cloud-mode-issues)
- [Performance Issues](#performance-issues)
- [Data & Metrics Issues](#data--metrics-issues)

---

## Standalone Server Issues

### CatOps Service Not Starting

**Symptoms:**
- `catops status` shows service is stopped
- No metrics being collected
- Telegram bot not responding

**Diagnosis:**
```bash
# Check service status
catops status

# Check if process is running
ps aux | grep catops

# Check for multiple instances
pgrep -a catops
```

**Solutions:**

**1. Force cleanup and restart:**
```bash
catops force-cleanup
catops restart
```

**2. Check logs:**
```bash
# Linux
journalctl -u catops --since "10 minutes ago" -f

# macOS
tail -f ~/Library/Logs/catops.log

# Check for common errors:
# - "permission denied" → Check file permissions
# - "address already in use" → Port conflict
# - "config file not found" → Configuration issue
```

**3. Verify configuration file:**
```bash
# Check config exists
ls -la ~/.catops/config.yaml

# View config
cat ~/.catops/config.yaml

# Fix permissions if needed
chmod 600 ~/.catops/config.yaml
```

**4. Reinstall:**
```bash
catops uninstall
curl -sfL https://get.catops.app/install.sh | bash
```

### Binary Not Found / Command Not Available

**Symptoms:**
- `catops: command not found`
- Shell can't find the binary

**Solutions:**

**1. Check if binary exists:**
```bash
# Find catops binary
which catops
find ~ -name catops -type f 2>/dev/null
```

**2. Add to PATH:**
```bash
# Linux
echo 'export PATH="$HOME/.catops:$PATH"' >> ~/.bashrc
source ~/.bashrc

# macOS
echo 'export PATH="$HOME/.catops:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

**3. Reinstall to proper location:**
```bash
curl -sfL https://get.catops.app/install.sh | bash
```

### High CPU Usage

**Symptoms:**
- CatOps using more than 10-20% CPU
- System slowdown

**Diagnosis:**
```bash
# Check CatOps CPU usage
top | grep catops
ps aux | grep catops
```

**Solutions:**

**1. Increase collection interval:**
```bash
# Collect less frequently
catops config interval=120  # Every 2 minutes instead of 1
catops restart
```

**2. Check for log spam:**
```bash
# Linux
journalctl -u catops --since "1 hour ago" | wc -l

# macOS
wc -l ~/Library/Logs/catops.log
```

**3. Update to latest version:**
```bash
catops update
```

### Auto-start Not Working

**Symptoms:**
- CatOps doesn't start after reboot
- Service disabled after system restart

**Diagnosis:**
```bash
# Check auto-start status
catops autostart status

# Linux - check systemd
systemctl status catops --user

# macOS - check launchd
launchctl list | grep catops
```

**Solutions:**

**1. Re-enable auto-start:**
```bash
catops autostart disable
catops autostart enable
catops restart
```

**2. Verify service file exists:**
```bash
# Linux
ls -la ~/.config/systemd/user/catops.service

# macOS
ls -la ~/Library/LaunchAgents/com.catops.agent.plist
```

**3. Manual service reload:**
```bash
# Linux
systemctl --user daemon-reload
systemctl --user enable catops
systemctl --user start catops

# macOS
launchctl unload ~/Library/LaunchAgents/com.catops.agent.plist
launchctl load ~/Library/LaunchAgents/com.catops.agent.plist
```

---

## Kubernetes Issues

### Pods in CrashLoopBackOff

**Symptoms:**
- CatOps pods constantly restarting
- Status shows `CrashLoopBackOff`

**Diagnosis:**
```bash
# Check pod status
kubectl get pods -n catops-system

# Check logs
kubectl logs -n catops-system -l app.kubernetes.io/name=catops --tail=100

# Check events
kubectl get events -n catops-system --sort-by='.lastTimestamp'
```

**Common Errors & Solutions:**

**1. "Failed to get metrics-server"**
```bash
# Install metrics-server
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# For Docker Desktop - allow insecure TLS
kubectl patch deployment metrics-server -n kube-system \
  --type='json' \
  -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--kubelet-insecure-tls"}]'

# Verify it works
kubectl top nodes
```

**2. "Failed to connect to Prometheus"**
```bash
# Check Prometheus is running
kubectl get pods -n catops-system | grep prometheus

# If Prometheus pods missing, enable it
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set prometheus.enabled=true

# Wait for Prometheus to be ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=prometheus -n catops-system --timeout=180s
```

**3. "Authentication failed" / 401 errors**
```bash
# Verify auth token is correct
kubectl get secret catops -n catops-system -o jsonpath='{.data.auth-token}' | base64 -d

# Update token if incorrect
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set auth.token=YOUR_CORRECT_TOKEN
```

### Pods Not Starting (Pending State)

**Symptoms:**
- Pods stuck in `Pending` state
- No pods running after installation

**Diagnosis:**
```bash
# Check pod status and events
kubectl describe pods -n catops-system

# Check node resources
kubectl top nodes
kubectl describe nodes
```

**Common Causes & Solutions:**

**1. Insufficient resources:**
```bash
# Reduce resource requests
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set resources.requests.memory=64Mi \
  --set resources.requests.cpu=50m
```

**2. Node selector mismatch:**
```bash
# Check if nodeSelector is set
helm get values catops -n catops-system | grep nodeSelector

# Remove nodeSelector if needed
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set nodeSelector=null
```

**3. Taints not tolerated:**
```bash
# Check node taints
kubectl describe nodes | grep Taints

# Add toleration for your taint
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set-json 'tolerations=[{"key":"your-taint","operator":"Exists","effect":"NoSchedule"}]'
```

### Metrics Not Appearing in Dashboard

**Symptoms:**
- Pods running successfully
- No metrics showing in web dashboard
- Nodes not appearing at catops.app

**Diagnosis:**
```bash
# Check pod logs for successful transmission
kubectl logs -n catops-system -l app.kubernetes.io/name=catops --tail=50 | grep -i "success\|sent\|error"

# Check network connectivity to backend
kubectl exec -n catops-system $(kubectl get pod -n catops-system -l app.kubernetes.io/name=catops -o name | head -1) -- \
  wget -O- --timeout=5 https://api.catops.app/health
```

**Solutions:**

**1. Verify auth token:**
```bash
# Get current token
kubectl get secret catops -n catops-system -o jsonpath='{.data.auth-token}' | base64 -d

# Update if wrong
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set auth.token=YOUR_CORRECT_TOKEN
```

**2. Check backend connectivity:**
```bash
# Test from inside pod
kubectl exec -n catops-system $(kubectl get pod -n catops-system -l app.kubernetes.io/name=catops -o name | head -1) -- \
  sh -c 'wget -O- --timeout=5 https://api.catops.app/health || echo "Connection failed"'
```

**3. Verify NetworkPolicy (if using):**
```bash
# Check for NetworkPolicy blocking egress
kubectl get networkpolicies -n catops-system

# Temporarily delete to test
kubectl delete networkpolicy -n catops-system --all
```

**4. Restart pods:**
```bash
kubectl rollout restart daemonset catops -n catops-system
kubectl rollout status daemonset catops -n catops-system
```

### Extended Metrics (Labels, Owners) Empty

**Symptoms:**
- Pod labels showing as empty in dashboard
- Owner information missing
- Container details not available

**Diagnosis:**
```bash
# Check if Prometheus is enabled
helm get values catops -n catops-system | grep -A 5 prometheus

# Check kube-state-metrics is running
kubectl get pods -n catops-system | grep kube-state-metrics

# Test kube-state-metrics endpoint
kubectl exec -n catops-system $(kubectl get pod -n catops-system -l app.kubernetes.io/name=catops -o name | head -1) -- \
  wget -qO- "http://catops-kube-state-metrics:8080/metrics" | grep "kube_pod_info" | head -5
```

**Solutions:**

**1. Enable Prometheus and kube-state-metrics:**
```bash
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set prometheus.enabled=true \
  --set kubeStateMetrics.enabled=true
```

**2. Wait for metrics to populate (takes 1-2 minutes):**
```bash
# Watch logs for Prometheus queries
kubectl logs -n catops-system -l app.kubernetes.io/name=catops -f | grep -i "prometheus\|labels"
```

**3. Verify kube-state-metrics has access:**
```bash
# Check RBAC permissions
kubectl auth can-i list pods --as=system:serviceaccount:catops-system:catops-kube-state-metrics -n default
```

### High Resource Usage in Kubernetes

**Symptoms:**
- CatOps using too much RAM/CPU
- Cluster performance degraded

**Diagnosis:**
```bash
# Check resource usage
kubectl top pods -n catops-system

# Check all resource requests/limits
kubectl describe pods -n catops-system | grep -A 5 "Limits:\|Requests:"
```

**Solutions:**

**1. Disable Prometheus (saves ~500 MB):**
```bash
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set prometheus.enabled=false \
  --set kubeStateMetrics.enabled=false \
  --set nodeExporter.enabled=false
```

**2. Reduce collection frequency:**
```bash
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set collection.interval=120  # 2 minutes instead of 1
```

**3. Lower resource limits:**
```bash
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set resources.limits.memory=256Mi \
  --set resources.limits.cpu=200m \
  --set prometheus.server.resources.limits.memory=256Mi \
  --set prometheus.server.resources.limits.cpu=250m
```

**4. Run on specific nodes only:**
```bash
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set nodeSelector.monitoring=enabled

kubectl label nodes node1 monitoring=enabled
```

### node-exporter Pods Failing (Docker Desktop)

**Symptoms:**
- node-exporter pods in `CrashLoopBackOff`
- Only on Docker Desktop
- Other pods running fine

**Cause:**
node-exporter tries to access host paths that don't exist in Docker Desktop

**Solution:**
```bash
# Disable node-exporter for Docker Desktop
helm upgrade catops oci://ghcr.io/mfhonley/catops/helm-charts/catops \
  --namespace catops-system \
  --reuse-values \
  --set nodeExporter.enabled=false

# CatOps works fine without it
```

---

## Telegram Bot Issues

### Bot Not Responding

**Symptoms:**
- Bot doesn't reply to commands
- No alerts being sent

**Diagnosis:**
```bash
# Check configuration
catops config show

# Look for Telegram settings
grep telegram ~/.catops/config.yaml
```

**Solutions:**

**1. Verify bot token is valid:**
- Open [@BotFather](https://t.me/botfather) in Telegram
- Send `/mybots` → Select your bot → Check token
- Update if needed:
  ```bash
  catops config token=YOUR_CORRECT_TOKEN
  catops restart
  ```

**2. Verify group ID is correct:**
```bash
# Check current group ID
catops config show | grep chat_id

# Get correct group ID:
# - Add @myidbot to your group
# - Send /getid in the group
# - Update CatOps:
catops config group=YOUR_CORRECT_GROUP_ID
catops restart
```

**3. Check bot is admin in group:**
- Open your Telegram group
- Group info → Administrators
- Verify your bot is listed
- If not, add it as admin

**4. Test bot manually:**
- Send `/start` to your bot directly (private message)
- If bot responds, issue is with group config
- If bot doesn't respond, issue is with token

### Bot Only Works in Private Chat

**Symptoms:**
- Bot responds to DMs
- Bot ignores group messages

**Cause:**
Bot doesn't have admin rights or privacy mode is enabled

**Solutions:**

**1. Make bot admin:**
- Group settings → Administrators → Add Administrator
- Select your bot
- Grant necessary permissions

**2. Disable privacy mode:**
- Open [@BotFather](https://t.me/botfather)
- Send `/mybots`
- Select your bot → Bot Settings → Group Privacy → Disable

### Alerts Not Being Sent

**Symptoms:**
- CatOps running
- No alerts when thresholds exceeded
- Bot works for manual commands

**Diagnosis:**
```bash
# Check alert thresholds
catops config show | grep threshold

# Check current metrics
catops status
```

**Solutions:**

**1. Lower thresholds to test:**
```bash
catops set cpu=10 mem=10 disk=10
# Should trigger alerts immediately
```

**2. Check logs for alert attempts:**
```bash
# Linux
journalctl -u catops --since "1 hour ago" | grep -i "alert\|telegram"

# macOS
tail -f ~/Library/Logs/catops.log | grep -i "alert\|telegram"
```

**3. Verify Telegram configuration:**
```bash
catops config show
catops restart
```

---

## Cloud Mode Issues

### Cannot Login / Authentication Failed

**Symptoms:**
- `catops auth login` fails
- "Invalid token" error

**Solutions:**

**1. Get fresh token:**
- Go to [catops.app](https://catops.app)
- Profile → Generate Auth Token
- Copy the new token
- Login:
  ```bash
  catops auth login YOUR_NEW_TOKEN
  ```

**2. Check internet connectivity:**
```bash
# Test backend connectivity
curl -I https://api.catops.app/health

# If fails, check firewall/proxy settings
```

**3. Clear old credentials:**
```bash
catops auth logout
rm ~/.catops/config.yaml
curl -sfL https://get.catops.app/install.sh | bash
catops auth login YOUR_TOKEN
```

### Metrics Not Appearing in Dashboard

**Symptoms:**
- Cloud Mode enabled
- No metrics showing at catops.app
- `catops auth info` shows "authenticated"

**Diagnosis:**
```bash
# Verify Cloud Mode is active
catops auth info

# Check config has auth_token and server_id
cat ~/.catops/config.yaml | grep -E "auth_token|server_id"

# Check service is running
catops status
```

**Solutions:**

**1. Restart service:**
```bash
catops restart
```

**2. Force re-registration:**
```bash
catops auth logout
catops auth login YOUR_TOKEN
catops restart
```

**3. Check logs for transmission errors:**
```bash
# Linux
journalctl -u catops --since "5 minutes ago" | grep -i "error\|failed\|sent"

# macOS
tail -100 ~/Library/Logs/catops.log | grep -i "error\|failed\|sent"
```

**4. Verify server appears in dashboard:**
- Login to [catops.app](https://catops.app)
- Check Servers page
- If server missing → Re-register
- If server shows "offline" → Check service status

### Server Showing as Offline

**Symptoms:**
- Server registered
- Shows as "offline" in dashboard
- No recent metrics

**Solutions:**

**1. Check service is running:**
```bash
catops status
catops restart
```

**2. Verify network connectivity:**
```bash
# Test backend connection
curl -v https://api.catops.app/health

# Check for proxy/firewall blocking
```

**3. Re-authenticate:**
```bash
catops auth logout
catops auth login YOUR_TOKEN
catops restart
```

---

## Performance Issues

### Slow Dashboard / UI

**Symptoms:**
- Dashboard loads slowly
- Metrics delayed
- UI feels sluggish

**Cause:**
Usually network latency or large number of servers

**Solutions:**

**1. Check browser console for errors:**
- F12 → Console tab
- Look for network errors or API failures

**2. Clear browser cache:**
- Hard refresh: Ctrl+Shift+R (Cmd+Shift+R on Mac)
- Clear site data in browser settings

**3. Reduce data load:**
- Use filters to show fewer servers
- Reduce time range for historical data

### High Memory Usage (Standalone)

**Symptoms:**
- CatOps using more than 50-100 MB RAM
- System memory pressure

**Diagnosis:**
```bash
# Check memory usage
ps aux | grep catops
top | grep catops
```

**Solutions:**

**1. Restart service:**
```bash
catops restart
```

**2. Update to latest version:**
```bash
catops update
```

**3. Check for memory leaks:**
```bash
# Monitor memory over time
watch -n 5 'ps aux | grep catops | grep -v grep'
```

---

## Data & Metrics Issues

### Missing Metrics / Incomplete Data

**Symptoms:**
- Some metrics showing as 0 or N/A
- Historical data has gaps

**Diagnosis:**
```bash
# Check service uptime
catops status

# Check logs for collection errors
# Linux
journalctl -u catops --since "1 hour ago" | grep -i "error\|failed"

# macOS
tail -100 ~/Library/Logs/catops.log | grep -i "error\|failed"
```

**Solutions:**

**1. Restart service:**
```bash
catops restart
```

**2. Check system metrics are available:**
```bash
# Test CPU reading
top -bn1 | head -10

# Test memory reading
free -h

# Test disk reading
df -h
```

**3. Verify permissions:**
```bash
# CatOps needs read access to /proc (Linux)
ls -la /proc/stat
ls -la /proc/meminfo
```

### Incorrect Metrics / Wrong Values

**Symptoms:**
- CPU showing 100% when system is idle
- Memory values don't match reality
- Disk usage incorrect

**Solutions:**

**1. Update CatOps:**
```bash
catops update
```

**2. Compare with system tools:**
```bash
# CPU
top -bn1 | head -10

# Memory
free -h

# Disk
df -h

# If values match system tools, issue is in dashboard
# If values don't match, issue is in collection
```

**3. Force cleanup:**
```bash
catops force-cleanup
catops restart
```

---

## Getting Further Help

If issues persist after trying these solutions:

**1. Gather diagnostic information:**
```bash
# Version
catops --version

# Configuration (remove sensitive data)
cat ~/.catops/config.yaml

# Service status
catops status

# Recent logs (last 50 lines)
# Linux:
journalctl -u catops --since "1 hour ago" --no-pager | tail -50

# macOS:
tail -50 ~/Library/Logs/catops.log

# For Kubernetes:
kubectl get pods -n catops-system
kubectl logs -n catops-system -l app.kubernetes.io/name=catops --tail=100
helm get values catops -n catops-system
```

**2. Contact support:**
- 💬 Telegram: [@mfhonley](https://t.me/mfhonley) - Fastest response
- 📧 Email: me@thehonley.org
- 🐛 GitHub Issues: [github.com/mfhonley/catops/issues](https://github.com/mfhonley/catops/issues)

**3. Include in your report:**
- Operating system and version
- CatOps version
- Error messages from logs
- Steps to reproduce the issue
- What you've already tried

---

**Last Updated:** 2025-01-15
