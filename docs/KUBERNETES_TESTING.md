# Kubernetes Integration Testing Guide

Пошаговая инструкция как протестировать Kubernetes интеграцию.

---

## 🎯 Что мы будем тестировать

1. ✅ Сборка Docker образа
2. ✅ Установка Helm chart в тестовый кластер
3. ✅ Проверка что pods запускаются
4. ✅ Проверка логов и сбора метрик
5. ✅ Тестирование отправки данных в backend

---

## 📋 Prerequisites

### 1. Установить необходимые инструменты:

```bash
# Docker Desktop (включает Kubernetes)
# https://www.docker.com/products/docker-desktop

# Или Minikube
brew install minikube

# Или Kind (Kubernetes in Docker)
brew install kind

# Helm
brew install helm

# kubectl
brew install kubectl
```

### 2. Запустить локальный Kubernetes кластер

**Вариант A: Docker Desktop (самый простой)**
1. Открыть Docker Desktop
2. Settings → Kubernetes → Enable Kubernetes
3. Подождать пока кластер запустится (~2 минуты)

**Вариант B: Minikube**
```bash
minikube start --driver=docker
```

**Вариант C: Kind**
```bash
kind create cluster --name catops-test
```

### 3. Проверить что кластер работает:

```bash
kubectl cluster-info
kubectl get nodes
```

Ожидаемый результат:
```
NAME             STATUS   ROLES           AGE   VERSION
docker-desktop   Ready    control-plane   5m    v1.27.2
```

---

## 🔧 Шаг 1: Установить metrics-server

Kubernetes connector использует Metrics API для получения метрик подов.

```bash
# Установить metrics-server
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# Для локального кластера (Docker Desktop/Minikube/Kind) нужен patch:
kubectl patch deployment metrics-server -n kube-system --type='json' \
  -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--kubelet-insecure-tls"}]'

# Подождать пока metrics-server запустится:
kubectl wait --for=condition=ready pod -l k8s-app=metrics-server -n kube-system --timeout=60s

# Проверить что metrics-server работает:
kubectl top nodes
```

Ожидаемый результат:
```
NAME             CPU(cores)   CPU%   MEMORY(bytes)   MEMORY%
docker-desktop   156m         3%     1456Mi          18%
```

Если выдает ошибку - подождите 1-2 минуты и попробуйте снова.

---

## 🐳 Шаг 2: Собрать Docker образ

### 2.1 Собрать образ локально:

```bash
cd /Users/honley/programs_honley/catops/cli

# Собрать образ
docker build -f Dockerfile.k8s -t catops/kubernetes-connector:dev .

# Проверить что образ создан
docker images | grep catops
```

### 2.2 Загрузить образ в кластер:

**Для Docker Desktop:**
```bash
# Образ уже доступен в кластере (используется локальный Docker registry)
```

**Для Minikube:**
```bash
minikube image load catops/kubernetes-connector:dev
```

**Для Kind:**
```bash
kind load docker-image catops/kubernetes-connector:dev --name catops-test
```

---

## 📦 Шаг 3: Установить Helm chart

### 3.1 Создать тестовый namespace:

```bash
kubectl create namespace catops-system
```

### 3.2 Установить CatOps connector:

```bash
cd /Users/honley/programs_honley/catops/cli

# Установить с локальным образом
helm install catops ./charts/catops \
  --set auth.token=test-token-12345 \
  --set backend.url=http://localhost:8000 \
  --set image.repository=catops/kubernetes-connector \
  --set image.tag=dev \
  --set image.pullPolicy=IfNotPresent \
  --namespace catops-system
```

**Параметры:**
- `auth.token` - любой тестовый токен (пока backend не готов)
- `backend.url` - URL вашего локального backend (или можно оставить default)
- `image.tag=dev` - используем локально собранный образ
- `image.pullPolicy=IfNotPresent` - не пытаться скачать из registry

### 3.3 Проверить что Helm chart установлен:

```bash
helm list -n catops-system
```

Ожидаемый результат:
```
NAME   	NAMESPACE      	REVISION	UPDATED                             	STATUS  	CHART        	APP VERSION
catops 	catops-system  	1       	2024-10-12 12:00:00.000000 +0300 MSK	deployed	catops-1.0.0 	1.0.0
```

---

## ✅ Шаг 4: Проверить что pods запустились

### 4.1 Посмотреть pods:

```bash
kubectl get pods -n catops-system
```

Ожидаемый результат (DaemonSet создаст 1 pod на каждой ноде):
```
NAME           READY   STATUS    RESTARTS   AGE
catops-xxxxx   1/1     Running   0          30s
```

### 4.2 Если pod в статусе Error/CrashLoopBackOff:

```bash
# Посмотреть описание пода
kubectl describe pod -n catops-system -l app.kubernetes.io/name=catops

# Посмотреть логи
kubectl logs -n catops-system -l app.kubernetes.io/name=catops --tail=50
```

**Частые проблемы:**

**Problem:** `ImagePullBackOff`
```
Solution: Образ не загружен в кластер. Выполните Шаг 2.2 снова.
```

**Problem:** `Invalid configuration: NODE_NAME is required`
```
Solution: Helm chart неправильно настроен. Удалите и переустановите:
kubectl delete pod -n catops-system --all
```

**Problem:** `metrics API is not accessible`
```
Solution: metrics-server не установлен. Выполните Шаг 1 снова.
```

---

## 📊 Шаг 5: Проверить логи

### 5.1 Смотреть логи в реальном времени:

```bash
kubectl logs -n catops-system -l app.kubernetes.io/name=catops -f
```

**Что вы должны увидеть:**

```
╔═══════════════════════════════════════╗
║   CatOps Kubernetes Connector v1.0.0   ║
╚═══════════════════════════════════════╝

📋 Configuration loaded successfully
   Backend URL: http://localhost:8000
   Node Name: docker-desktop
   Namespace: catops-system
   Collection Interval: 60s

🔌 Connecting to Kubernetes API...
✅ Connected to Kubernetes API
✅ Kubernetes API is healthy
🚀 Starting metrics collection...

📊 Collecting metrics...
✅ Metrics collected and sent successfully (took 1.2s)
   Node metrics: CPU=45.0%, Memory=70.0%, Disk=50.0%
   Pods on this node: 12
   Cluster: 1/1 nodes ready, 12/12 pods running
```

### 5.2 Если видите ошибки:

**Error:** `Failed to send metrics: connection refused`
```
Это НОРМАЛЬНО на данном этапе!
Backend endpoint еще не создан.
```

**Error:** `Failed to collect pod metrics`
```
Проверьте metrics-server: kubectl top pods
Если metrics-server не работает - выполните Шаг 1 снова.
```

**Error:** `Server not found or access denied`
```
RBAC проблема. Проверьте ClusterRole:
kubectl get clusterrole catops-catops -o yaml
```

---

## 🧪 Шаг 6: Тестирование функциональности

### 6.1 Проверить RBAC permissions:

```bash
# Проверить что ServiceAccount создан
kubectl get serviceaccount -n catops-system catops

# Проверить ClusterRole
kubectl get clusterrole catops-catops

# Проверить ClusterRoleBinding
kubectl get clusterrolebinding catops-catops
```

### 6.2 Проверить что connector может читать метрики:

```bash
# Exec в pod
POD=$(kubectl get pods -n catops-system -l app.kubernetes.io/name=catops -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it -n catops-system $POD -- sh

# Внутри пода (не будет работать т.к. alpine минимальный):
# Вместо этого просто проверьте что pod запустился и работает
```

### 6.3 Создать тестовые pods для мониторинга:

```bash
# Создать тестовый deployment
kubectl create deployment nginx --image=nginx --replicas=3

# Подождать пока pods запустятся
kubectl wait --for=condition=ready pod -l app=nginx --timeout=60s

# Проверить что pods видны
kubectl get pods

# Теперь в логах CatOps должно быть больше pods:
kubectl logs -n catops-system -l app.kubernetes.io/name=catops --tail=20
```

Вы должны увидеть:
```
   Pods on this node: 15  # было 12, теперь +3 nginx pods
```

---

## 🔄 Шаг 7: Тестирование обновлений

### 7.1 Обновить configuration:

```bash
# Изменить collection interval на 30 секунд
helm upgrade catops ./charts/catops \
  --set auth.token=test-token-12345 \
  --set backend.url=http://localhost:8000 \
  --set collection.interval=30 \
  --set image.repository=catops/kubernetes-connector \
  --set image.tag=dev \
  --set image.pullPolicy=IfNotPresent \
  --namespace catops-system

# Проверить что pods перезапустились
kubectl get pods -n catops-system -w
```

### 7.2 Проверить что новый интервал работает:

```bash
kubectl logs -n catops-system -l app.kubernetes.io/name=catops -f
# Метрики должны собираться каждые 30 секунд вместо 60
```

---

## 🧹 Шаг 8: Cleanup (удаление)

### 8.1 Удалить CatOps:

```bash
helm uninstall catops -n catops-system
```

### 8.2 Удалить namespace:

```bash
kubectl delete namespace catops-system
```

### 8.3 Удалить тестовые pods:

```bash
kubectl delete deployment nginx
```

### 8.4 Удалить кластер (если использовали Kind/Minikube):

```bash
# Kind
kind delete cluster --name catops-test

# Minikube
minikube delete
```

---

## 📝 Шаг 9: Следующие шаги

После успешного тестирования:

### 9.1 Создать backend endpoint

Нужно создать endpoint в Python backend:
```python
# back/app/routers/cli/data.py
@router.post("/kubernetes/metrics")
async def upload_kubernetes_metrics(...):
    pass
```

### 9.2 Протестировать с реальным backend

```bash
# Запустить backend локально
cd /Users/honley/programs_honley/catops/back
python -m uvicorn app.main:app --reload

# Переустановить с реальным URL
helm upgrade catops ./charts/catops \
  --set auth.token=REAL_TOKEN \
  --set backend.url=http://host.docker.internal:8000 \
  --namespace catops-system
```

**Note:** `host.docker.internal` позволяет Docker контейнерам обращаться к localhost хоста.

### 9.3 Опубликовать Docker образ

```bash
# Build multi-arch образ
docker buildx build --platform linux/amd64,linux/arm64 \
  -f Dockerfile.k8s \
  -t ghcr.io/catops/cli/kubernetes-connector:1.0.0 \
  --push .
```

### 9.4 Merge в main

Когда всё протестировано:
```bash
git checkout main
git merge k8s
git push origin main
```

---

## 🐛 Troubleshooting

### Logs не показывают метрики

**Проблема:** Pod запустился но нет логов о сборе метрик

**Решение:**
```bash
# Проверить environment variables
kubectl exec -n catops-system $POD -- env | grep CATOPS

# Должны быть:
CATOPS_BACKEND_URL=...
CATOPS_AUTH_TOKEN=...
NODE_NAME=...
NAMESPACE=...
```

### Metrics API errors

**Проблема:** `metrics API is not accessible`

**Решение:**
```bash
# Проверить metrics-server
kubectl get deployment metrics-server -n kube-system

# Перезапустить metrics-server
kubectl rollout restart deployment metrics-server -n kube-system

# Подождать 1-2 минуты
kubectl top nodes
```

### RBAC Permission errors

**Проблема:** `pods is forbidden: User "system:serviceaccount:catops-system:catops" cannot list`

**Решение:**
```bash
# Проверить ClusterRoleBinding
kubectl describe clusterrolebinding catops-catops

# Пересоздать RBAC
helm uninstall catops -n catops-system
helm install catops ./charts/catops ...
```

---

## ✅ Checklist успешного тестирования

- [ ] Кластер запущен и доступен
- [ ] metrics-server установлен и работает
- [ ] Docker образ собран локально
- [ ] Helm chart установлен без ошибок
- [ ] Pods в статусе Running
- [ ] Логи показывают сбор метрик
- [ ] Метрики собираются каждые 60 секунд
- [ ] Pod metrics корректны (видны nginx pods)
- [ ] Cluster metrics корректны
- [ ] Обновление через helm upgrade работает
- [ ] Cleanup прошел успешно

---

**Готово! После выполнения всех шагов у вас будет протестированная Kubernetes интеграция!** 🎉
