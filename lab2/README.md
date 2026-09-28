# НЕТ ПОВЕСТИ ПЕЧАЛЬНЕЕ НА СВЕТЕ,
# ЧЕМ ПОВЕСТЬ О ТЁМЕ, ОБСИРВОБЕТИ И МОНИТОРЕТЕ

<img src="imgs/11.png" alt="w" width="" height="">


<img src="imgs/image.png" alt="meme1" width="500" height="250">


# ЧАСТЬ 0

    Решил использовать minikube.
    Скачал minikube и kubectl.

    Написал код. Написал Докер файл, собрал образ и загрузил образ в кубик

<img src="imgs/1.png" alt="1" width="400" height="275">

    Далее нужно было написать манифест Deployment и Service
    Загрузить манифест в кубик.

<img src="imgs/2.png" alt="2" width="400" height="275">

```yaml
kind: Deployment
spec:
  replicas: 1
  selector:
    spec:
      containers:
        - name: api
          image: api:latest
          imagePullPolicy: Never
          ports:
            - name: http
              containerPort: 1414
          env:
            - name: OTEL_EXPORTER_OTLP_ENDPOINT
              value: "jaeger.monitoring:4317"
            - name: OTEL_SERVICE_NAME
              value: "api"
            - name: SELF_URL
              value: "http://localhost:1414"
---
apiVersion: v1
kind: Service
spec:
  selector:
    app: api
  ports:
    - name: http
      port: 1414
      targetPort: 1414
```

Что для чего нужно:
 - Deployment - запускает контейнер с приложением из указыванного образа
 - Service - настраивает порты для сети внутри кластера

Далее нам нужен скачать HELM
Тут скриншота не будет потому "HAPPY HELMING"

    А что такое HELM?
    helm - менеджер пакетов для кубика, тут я могу сравнить только с maven для java и spring 

Остался только одна проблемка если перейти на localhost:1414 то.... 
Ничего не получится потому что порты прокинуты только внутри кластера
Я настраивал порты идентично тем что есть внутри с помощью `kubectl port-forward svc/api 1414:1414 &`
Для все сервисов далее так же пришлось прокидывать port-forward (((

# ЧАСТЬ 1 Метрики (Prometheus + Grafana)

СПАСИБО ЧТО ЕСТЬ HELM!!!
Всё устанавливаю через него. 


<img src="imgs/3.png" alt="3" width="600" height="300">

    Запускаем и проверяем запустились ли они

<img src="imgs/4.png" alt="4" width="600" height="300">

    После чего надо прописать манифест servicemonitor
    Нужен он для того чтобы показать Prometheus для того что он искал сервис api и с портом 1414 и обращался на ручку /metrics

<img src="imgs/5.png" alt="5" width="600" height="300">

    GRAFANA же в свою очереь настраивается сама нужно только прокинуть 
    `kubectl port-forward -n monitoring svc/kps-grafana 3000:80`
    Она берёт данные из Prometheus

<img src="imgs/6.png" alt="6" width="800" height="500">

    Вот такая красота получилась 
    А именно вот так:

```promql

sum(rate(http_requests_total[1m])) by (path)

100 * (
    sum(rate(http_requests_total{status=~"5.."}[5m])) by (path)
    /
    sum(rate(http_requests_total[5m])) by (path)
)

histogram_quantile(0.95,sum(rate(http_request_duration_seconds_bucket[5m])) by (le, path))

```


<img src="imgs/image1.png" alt="meme2" width="1000" height="300">


# ЧАСТЬ 2 ЛОГ(К)И

<img src="imgs/image3.png" alt="meme3" width="400" height="250">

    Матрики метриками растут падают, но как нам понимать что именно произошло и почему что то идёт не так
    Логи как раз и нужны нам чтобы это ПОНИМТЬ 

Установка:

```bash
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update

helm install loki grafana/loki-stack \
--namespace monitoring \
--set promtail.enabled=true \
--set loki.persistence.enabled=false \
--set grafana.enabled=false
```

# ЧАСТЬ 3


# ЧАСТЬ 4

