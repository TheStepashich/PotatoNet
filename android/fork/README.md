# Форк EasyTier Android — кастомный DNS

Патч поверх официального репозитория EasyTier **v2.4.5** (та же версия, что
использует pnet в качестве ядра). Добавляет в Android-приложение возможность
задать свой DNS-сервер для сети — например `10.126.0.1` (DNS pnet).

## Что меняет патч

| Файл | Изменение |
| --- | --- |
| `easytier-web/frontend-lib/src/types/network.ts` | Поле `dns?: string` в `NetworkConfig` + значение по умолчанию `''` |
| `easytier-web/frontend-lib/src/components/Config.vue` | Поле ввода `DNS` в базовых настройках сети |
| `easytier-gui/src/composables/mobile_vpn.ts` | Передаёт `dns` в `start_vpn` (→ `addDnsServer`) и добавляет маршрут `dns/32` через туннель |
| `easytier-web/frontend-lib/src/locales/en.yaml`, `cn.yaml` | Ключи перевода |

Пустое поле = поведение оригинала (`114.114.114.114`). Если ввести адрес
с префиксом (`10.126.0.1/32`) — в DNS уйдёт чистый IP, а `/32` попадёт в маршруты.

## Готовые артефакты

Собранные APK/AAB лежат в [build/](build/) (загитигнорены, 32–72 МБ):

- `easytier-custom-dns-v2.4.5-universal.apk` — universal (4 ABI), можно ставить
  `adb install` на устройство.
- `easytier-custom-dns-v2.4.5-universal.aab` — для Play Store.

## Сборка

Воспроизводимая сборка — `./build.sh` (внутри всё: клон, патч, deps, фронтенд,
Rust, Gradle). Требует Android SDK (platform-34, build-tools 34, NDK r26), JDK 21,
rustup + Android-таргеты, pnpm.

Вручную:

```bash
git clone --depth 1 --branch v2.4.5 https://github.com/EasyTier/EasyTier.git
cd EasyTier
git apply ../easytier-android-custom-dns-v2.4.5.patch

pnpm install
(cd easytier-web/frontend-lib && ./node_modules/.bin/vite build)
(cd easytier-gui && ./node_modules/.bin/vite build)

# bindgen (kcp-sys) должен видеть sysroot NDK, иначе падает на /usr/include
export SYSROOT=$NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64/sysroot
export BINDGEN_EXTRA_CLANG_ARGS_aarch64_linux_android="--target=aarch64-linux-android --sysroot=$SYSROOT -I$SYSROOT/usr/include -I$SYSROOT/usr/include/aarch64-linux-android"
# (аналогично для armv7/i686/x86_64, см. build.sh)

cd easytier-gui
./node_modules/.bin/tauri android build --config '{"build":{"beforeBuildCommand":""}}'
```

Изменения затрагивают только фронтенд GUI — пересборки ядра/плагина не требуется.

## Проверка (что делалось)

- `vue-tsc --noEmit` в `easytier-web/frontend-lib` и `easytier-gui` — чисто.
- Новых ошибок ESLint нет (оставшиеся `no-console` были в оригинале).
- APK собран (4 ABI) и проверен: в бандле внутри `libapp_lib.so` (ассеты Tauri
  сжаты brotli) присутствуют ключи `dns_placeholder`, текст `Custom DNS server`,
  `dns:""` в дефолтном конфиге и вызов `start_vpn({..., dns, ...})`. Старые
  чанки в мапе ассетов отсутствуют.

## Как пользоваться с pnet

1. В приложении создайте сеть с теми же `network_name` / `network_secret`, что и у
   pnet-ноды.
2. В поле **DNS** укажите адрес DNS-сервера pnet (по умолчанию `10.126.0.1`).
3. Подключитесь — Android будет резолвить имена через DNS внутри туннеля.
