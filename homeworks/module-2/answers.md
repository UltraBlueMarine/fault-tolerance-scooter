### Задание 1

**Найдите загрузку данных и исправьте readiness. Проверяемая ручка — GET /v1/pricing/tariffs, RPC — PricingService/ListTariffs.**

[init.go:233-238](../../services/pricing/internal/init.go#L233-L238)

```go
a.healthCheck.AddReadinessCheck("ready", func() error {
	if a.storage.Tariffs.IsLoaded() {
		return nil
	}
	return fmt.Errorf("Тарифы не загружены в базу данных, приложение не готово.")
})
```

```bash
ТРАФИК: с тарифами=1329 · пустых HTTP 200=0 · ошибок=0
OK Rollout завершился: все новые реплики готовы, старые удалены
OK Тарифы при rollout: map[full:1329]
OK Новая реплика ждала готовности: наблюдение Pod Ready
Изолированная проверка: missing-file
OK Readiness: missing-file: {
                "goroutines": "success",
                "ready": "Тарифы не загружены в базу данных, приложение не готово.",
                "started": "success",
                "termination": "success"
}

OK Ошибка файла видна в логах: журнал загрузки Pricing
OK Liveness при missing-file: 200
OK Прогрев не перезапускает Pod: Running
```

**Почему трафиком здесь управляет readiness?**

Кубер через GET-запросы проверяет readiness, то есть готовность сервера принимать запросы. Когда по каким-то причинам readiness перестаёт быть True, кубер перестаёт направлять на под трафик и ждёт, пока статус изменится.

**Чем отличаются задачи startup и liveness?**

Startup отвечает за запуск приложения со всеми его зависимостями. Startup будет возвращать ошибку 503, пока не будут подключены все зависимости (БД, брокеры и т. д.). Liveness проверяет, живо ли приложение и не зависло ли оно из-за какой-то ошибки.

### Задание 2

**Что означает флаг и чем отличается от started?**

Когда сервер запущен и все зависимости проинициализированы, `a.started` меняется на 1 (true).

[app.go:141](../../services/pricing/internal/app.go#L141)

```go
	atomic.StoreInt32(&a.started, 1)
```

**Кто его изменяет и читает? Почему доступ из разных горутин должен быть безопасным?**

Читают флаг в health checks, а меняют один раз после инициализации приложения, как показано в коде. Флаг пишет одна горутина, а читают горутины health check одновременно с ней. Без `atomic` это гонка данных (data race), поэтому используются `atomic.StoreInt32` и `atomic.LoadInt32`.

**Как обрабатываются SIGTERM и SIGINT? Почему нельзя обработать SIGKILL?**

Все эти сигналы предназначены для завершения процесса. SIGINT отправляется локальному процессу (Ctrl+C), а SIGTERM отправляют менеджеры процессов (например, k8s). SIGKILL — это принудительное завершение процесса со стороны ОС, процесс не может его перехватить.

[closer.go:35-43](../../services/pricing/internal/pkg/closer/closer.go#L35-L43) — здесь перехватываются SIGINT и SIGTERM.

**Что флаг делает при graceful shutdown, а кто завершает уже принятые запросы?**

`a.terminated` становится 1, readiness начинает возвращать 503, кубер это видит и перестаёт отправлять запросы на под.

Здесь завершаются все уже принятые запросы: [init.go:118-125](../../services/pricing/internal/init.go#L118-L125)

```go
    done := make(chan struct{})
            go func() {
                err := a.mainServer.Stop(gracefulCtx)
                if err != nil {
                    slog.Error("Не удалось остановить основной сервер", "error", err)
                }
                close(done)
            }()
```

**Гарантирует ли порядок регистрации функций в Closer порядок выполнения?**

Нет. Все функции из `funcs` запускаются в отдельных горутинах: [closer.go:69](../../services/pricing/internal/pkg/closer/closer.go#L69)

***Короткие ответы со ссылками на код и подтверждённое поведение флага. Переписывать весь graceful shutdown не требуется.***

[closer.go:39](../../services/pricing/internal/pkg/closer/closer.go#L39) — сюда приходит сигнал SIGINT или SIGTERM.

Логи:

[init.go:250](../../services/pricing/internal/init.go#L250) 2026-11-17T22:54:27.439494761Z {"time":"2026-11-17T22:54:27.439372553Z","level":"INFO","msg":"Получен сигнал остановки","service":"pricing","timeout":10000000000}

[app.go:126](../../services/pricing/internal/app.go#L126) 2026-11-17T22:54:30.461319846Z {"time":"2026-11-17T22:54:30.461195721Z","level":"ERROR","msg":"Ошибка основного сервера","error":"http: Server closed"}

[init.go:130](../../services/pricing/internal/init.go#L130) 2026-11-17T22:54:30.461335846Z {"time":"2026-11-17T22:54:30.461277471Z","level":"INFO","msg":"Основной сервер остановлен","service":"pricing"}


### Задание 3

***До Carbon:***

pricing — 01:55:51
POD                       STATUS   LIVE  READY  RESTARTS  READINESS
pricing-64995464bf-mbpx6  Running  true  true   0         {"cordon":"success","goroutines":"success","ready":"success","started":"success","termination":"success"}
pricing-64995464bf-mf2rt  Running  true  true   0         {"cordon":"success","goroutines":"success","ready":"success","started":"success","termination":"success"}
pricing-64995464bf-s57zk  Running  true  true   0         {"cordon":"success","goroutines":"success","ready":"success","started":"success","termination":"success"}

***После Carbon pricing-64995464bf-mf2rt:***

POD                       STATUS   LIVE  READY  RESTARTS  READINESS
pricing-64995464bf-mbpx6  Running  true  true   0         {"cordon":"success","goroutines":"success","ready":"success","started":"success","termination":"success"}
pricing-64995464bf-mf2rt  Running  true  false  0         {"cordon":"Cordon активен","goroutines":"success","ready":"success","started":"success","termination":"success"}
pricing-64995464bf-s57zk  Running  true  true   0         {"cordon":"success","goroutines":"success","ready":"success","started":"success","termination":"success"}
