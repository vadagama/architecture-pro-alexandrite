# Jaeger в Minikube с сервисами

## Описание
Развертывание Jaeger в Minikube с двумя Go-сервисами:
1. **service-a** (сервис заказов) — принимает GET-запрос, создаёт заказ, вызывает service-b для расчёта стоимости
2. **service-b** (сервис расчёта) — принимает GET-запрос, рассчитывает стоимость с учётом скидки

Оба сервиса используют OpenTelemetry SDK для отправки трейсов в Jaeger через OTLP gRPC. Вызов service-a → service-b попадает в один трейс благодаря propagation контекста через HTTP-заголовки (W3C TraceContext).

## Требования
- Minikube
- kubectl
- Docker

## Установка

### 1. Запуск Minikube 
```bash
minikube start --addons=ingress 
```

### 2. Установка cert-manager
```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.3/cert-manager.yaml
kubectl wait --for=condition=Available deployment --all -n cert-manager --timeout=120s
```

### 3. Развертывание Jaeger
```bash
kubectl create namespace observability
kubectl create -f https://github.com/jaegertracing/jaeger-operator/releases/download/v1.51.0/jaeger-operator.yaml -n observability
kubectl wait --for=condition=Available deployment/jaeger-operator -n observability --timeout=120s
kubectl apply -f k8s/jaeger-instance.yaml
```

### 4. Сборка и деплой сервисов
```bash
# Сборка образов внутри Minikube
minikube image build -t service-a:latest services/service-a/
minikube image build -t service-b:latest services/service-b/

# Развертывание
kubectl apply -f k8s/services.yaml
```

### 5. Проверка что поды запущены
```bash
kubectl get pods
# Ожидаем статус Running у service-a, service-b и simplest-*
```

## Проверка работы

### Доступ к Jaeger UI
```bash
kubectl port-forward svc/simplest-query 16686:16686
```
Откройте в браузере: http://localhost:16686

### Тестирование сервисов
```bash
# Вызов service-a, который вызывает service-b
kubectl exec -it $(kubectl get pods -l app=service-a -o jsonpath='{.items[0].metadata.name}') -- wget -qO- http://service-a:8080
```

В ответе будет JSON с order_id, status и cost (результат расчёта от service-b).

### Просмотр трейсов
1. Откройте Jaeger UI (http://localhost:16686)
2. В выпадающем списке Service выберите `service-a`
3. Нажмите **Find Traces**
4. Каждый трейс содержит спаны от обоих сервисов:
   - `service-a: order-service` — входящий HTTP-запрос
   - `service-a: create-order` — бизнес-логика создания заказа
   - `service-b: calculate-cost` — вычисление стоимости

## Структура проекта
```
k8s-trace/
├── k8s/
│   ├── jaeger-instance.yaml    # Jaeger CRD (allInOne, in-memory storage)
│   └── services.yaml           # Deployments и Services для service-a и service-b
└── services/
    ├── service-a/              # Go-сервис заказов (OpenTelemetry + OTLP)
    │   ├── main.go
    │   ├── go.mod / go.sum
    │   └── Dockerfile
    └── service-b/              # Go-сервис расчёта (OpenTelemetry + OTLP)
        ├── main.go
        ├── go.mod / go.sum
        └── Dockerfile
```

## Как работает трейсинг
- Оба сервиса инициализируют OpenTelemetry TracerProvider с OTLP gRPC экспортером
- service-a использует `otelhttp.NewTransport` для автоматической инъекции trace context в исходящие HTTP-запросы
- service-b использует `otelhttp.NewHandler` для автоматического извлечения trace context из входящих запросов
- Propagation формат — W3C TraceContext (заголовки `traceparent` / `tracestate`)
- Jaeger Operator принимает трейсы на `simplest-collector:4317` (OTLP gRPC)
