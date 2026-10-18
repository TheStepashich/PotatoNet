# PotatoNet — простой P2P VPN

Нативное приложение для создания приватных VPN-сетей поверх
[EasyTier](https://github.com/EasyTier/EasyTier). Аналог Radmin VPN, но проще:
ввели имя сети, пароль и имя устройства — и вы в одной сети с друзьями.

Приложение — один исполняемый файл на [Wails v2](https://wails.io)
(Go + нативный webview), интерфейс встроен в бинарник.

## Интерфейс

Всё максимально просто, никаких лишних настроек:

- **Имя сети** — сети с одинаковым именем и паролем видят друг друга.
- **Пароль** — общий секрет сети.
- **Имя устройства (hostname)** — как вас видят другие; можно оставить пустым.
- **Список устройств** — таблица подключённых узлов (IP, имя, тип соединения,
  задержка, потери, трафик, NAT).
- **Несколько сетей** — можно добавить сколько угодно сетей; каждая
  отображается отдельной карточкой со своим списком устройств.
- **Опции** — автозапуск при входе в систему: приложение стартует и сразу
  подключается к сохранённым сетям.

## Сборка

Нужны Go, `curl` и `unzip`; для сборки GUI — Wails v2 и зависимости webview
(на Linux — `webkit2gtk-4.1`):

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

`makefile` сам находит `wails` в `PATH` или в `$(go env GOPATH)/bin`.

```bash
make run        # собрать и запустить (Linux)
make build      # build/dist-linux/
make test       # тесты
```

Результат — папка `build/dist-linux` с исполняемым файлом `PotatoNet` и
комплектом EasyTier рядом.

### Версия и bootstrap

Версия хранится в одном файле `VERSION` и подставляется во все сборки:
бинарник (`PotatoNet -version`), установщик Windows, deb/rpm и AppImage.

Свой bootstrap-сервер задаётся при сборке, без правки кода:

```bash
make build BOOTSTRAP=udp://my.server:11010
```

Несколько серверов — через запятую. Для уже собранного приложения то же можно
задать переменной `POTATONET_BOOTSTRAP` или в `config.yml`.

### Windows

Кросс-сборка из Linux (нужны `mingw-w64`, `curl`, `unzip`, `zip`, `upx`):

```bash
make build PLATFORM=windows      # папка build/dist-win/
make installer PLATFORM=windows  # установщик build/PotatoNet-setup-x64.exe
```

`easytier-core.exe` намеренно **не** сжимается UPX: после UPX у Rust-бинарника
ломаются таблицы исключений и он падает с `0xc0000005`.

### AppImage

```bash
make appimage   # build/PotatoNet-linux-x64.AppImage
```

Релизные сборки (Windows + Linux) автоматически собираются в GitHub Actions по
пушу тега `v*`.
