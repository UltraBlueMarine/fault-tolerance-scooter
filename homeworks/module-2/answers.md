### Задание 1

**Найдите загрузку данных и исправьте readiness. Проверяемая ручка — GET /v1/pricing/tariffs, RPC — PricingService/ListTariffs.**

[init.go:216-221](../../services/pricing/internal/init.go#L216-L221)

```go
a.healthCheck.AddReadinessCheck("ready", func() error {
	if a.storage.Tariffs.IsLoaded() {
		return nil
	}
	return fmt.Errorf("Тарифы ещё не загружены")
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

Кубер проверяет через GET запросы readiness сервера принимать запросы, когда по каким-то причинам readiness перестает быть True кубер перестает давать на него трафик и ждет пока статус измениться.

**Чем отличаются задачи startup и liveness?**

Startup отвечает за запуск приложения со всеми ее зависимостями. Startup будет выдавать 503 ошибку пока не будут подключены все зависимости(БД, Брокеры и тд.). Liveness проверяет живое ли приложение не зависло ли оно от какой-то ошибки.

### Задание 2



**Что означает флаг и чем отличается от started?**
Когда сервер запущен и все зависимости проиницилизированы, меняется статус a.started на True.

[app.go:140](../../services/pricing/internal/app.go#L140)

```go
	atomic.StoreInt32(&a.started, 1)
```

**Кто его изменяет и читает? Почему доступ из разных горутин должен быть безопасным?**
читают в healthchecks
меняют после инциализации приложения один раз, как показано в коде;

**Как обрабатываются SIGTERM и SIGINT? Почему нельзя обработать SIGKILL?**
Все комманды предназначены для завершения процесса. SIGTERM для локального процесса и SIGTERM уже для менеджеров процессов (k8s как пример); SIGKILL это уже со стороны OS происходит убийство процесса, обычный процесс не может выловить.

[closer.go:35-43](../../services/pricing/internal/pkg/closer/closer.go#L35-L43) тут ловятся SIGINT или SIGTERM;
И дальше уже за

Что флаг делает при graceful shutdown, а кто завершает уже принятые запросы?
a.terminated становится 1, и куб видит healthcheck отправляет 500 ошибку и перестает отправлять реквесты.

Тут завершаются уже все принятые запросы: [init.go:118-125](../../services/pricing/internal/init.go#L118-L125)

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

Гарантирует ли порядок регистрации функций в Closer порядок выполнения?
Нет. Все функции с funcs запускаются в отдельные горутины: [closer.go:69](../../services/pricing/internal/pkg/closer/closer.go#L69)

***Короткие ответы со ссылками на код и подтверждённое поведение флага. Переписывать весь graceful shutdown не требуется.***

[closer.go:39](../../services/pricing/internal/pkg/closer/closer.go#L39) Сюда прилетает сигнал от SIGINT или SIGTERM

Logs:
[init.go:233](../../services/pricing/internal/init.go#L233) 2026-11-17T22:54:27.439494761Z {"time":"2026-11-17T22:54:27.439372553Z","level":"INFO","msg":"Получен сигнал остановки","service":"pricing","timeout":10000000000}

[app.go:125](../../services/pricing/internal/app.go#L125) 2026-11-17T22:54:30.461319846Z {"time":"2026-11-17T22:54:30.461195721Z","level":"ERROR","msg":"Ошибка основного сервера","error":"http: Server closed"}

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