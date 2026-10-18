PLATFORM   ?= linux
ARCH       ?= amd64
ET_VERSION ?= 2.4.5
VERSION    ?= $(shell cat VERSION 2>/dev/null || echo 0.0.0)
BOOTSTRAP  ?= udp://bootstrap.potatom.net:11010

GO        = go
GOBIN     = $(shell $(GO) env GOPATH)/bin
WAILS    ?= $(shell command -v wails 2>/dev/null || echo $(GOBIN)/wails)
BUILD_DIR = ./build

ifeq ($(PLATFORM),windows)
WAILS_PLATFORM = windows/amd64
APP_BIN     = PotatoNet.exe
OS_NAME = win
ET_DIRNAME  = easytier-windows-x86_64
ET_CORE     = easytier-core.exe
ET_CLI      = easytier-cli.exe
export CC   = x86_64-w64-mingw32-gcc
export CXX  = x86_64-w64-mingw32-g++
WAILS_ENV   =
WAILS_TAGS  =
else
WAILS_PLATFORM = linux/amd64
APP_BIN     = PotatoNet
OS_NAME = linux
ET_DIRNAME  = easytier-linux-x86_64
ET_CORE     = easytier-core
ET_CLI      = easytier-cli
# webkit2gtk-4.1: современный ABI, libjxl 0.12 (в 4.0 на Arch/rolling — битая
# зависимость libjxl.so.0.11). 4.1 присутствует во всех актуальных дистрибутивах.
WAILS_TAGS  = -tags webkit2_41
WAILS_ENV   =
endif

ICON_PNG = assets/icon.png
ICON_ICO = assets/icon.ico

ET_ZIP = $(ET_DIRNAME)-v$(ET_VERSION).zip
DIST   = $(BUILD_DIR)/dist-$(OS_NAME)

# Версия (из файла VERSION) и bootstrap подставляются в бинарник без правки кода:
#   make build VERSION=1.2.3 BOOTSTRAP=udp://my.server:11010
LDFLAGS = -X potatonet/src.Version=$(VERSION) -X potatonet/src.BootstrapPeer=$(BOOTSTRAP)

APPIMAGE_TOOL = $(BUILD_DIR)/tools/appimagetool

.PHONY: all build run package appimage installer test clean

all: build

# Иконки: из PNG собирается многоразмерный ICO (для установщика) и копия
# build/appicon.png, из которой Wails делает иконку EXE. Пересобираются
# автоматически, когда assets/icon.png новее.
$(ICON_ICO): $(ICON_PNG)
	@if command -v magick >/dev/null 2>&1; then \
		magick $(ICON_PNG) -define icon:auto-resize=256,128,96,64,48,32,24,16 $(ICON_ICO); \
	else \
		echo "magick не найден: оставляю существующий $(ICON_ICO)"; \
	fi

$(BUILD_DIR)/appicon.png: $(ICON_PNG)
	@mkdir -p $(BUILD_DIR)
	@cp -f $(ICON_PNG) $@

# Собирает Wails-приложение и раскладывает самодостаточный дистрибутив в
# build/dist-<platform>: бинарник PotatoNet + комплект EasyTier рядом с ним.
build: $(ICON_ICO) $(BUILD_DIR)/appicon.png
	@command -v $(WAILS) >/dev/null 2>&1 || { \
		echo "Не найден 'wails' ($(WAILS))."; \
		echo "Установите: go install github.com/wailsapp/wails/v2/cmd/wails@latest"; \
		exit 1; \
	}
	@# Wails кэширует build/windows/icon.ico и не обновляет его при смене
	@# appicon.png — удаляем, чтобы иконка EXE всегда соответствовала исходнику.
	@if [ "$(PLATFORM)" = "windows" ]; then rm -f $(BUILD_DIR)/windows/icon.ico; fi
	@echo "Сборка PotatoNet ($(WAILS_PLATFORM))..."
	@$(WAILS_ENV) $(WAILS) build -s -clean -platform $(WAILS_PLATFORM) $(WAILS_TAGS) -ldflags "$(LDFLAGS)" -o $(APP_BIN)
	@rm -rf $(DIST)
	@mkdir -p $(DIST)/$(ET_DIRNAME)
	@cp -f $(BUILD_DIR)/bin/$(APP_BIN) $(DIST)/$(APP_BIN)
	@if [ ! -f $(BUILD_DIR)/$(ET_ZIP) ]; then \
		echo "Скачиваю EasyTier $(ET_VERSION)..."; \
		curl -fL -o $(BUILD_DIR)/$(ET_ZIP) \
			https://github.com/EasyTier/EasyTier/releases/download/v$(ET_VERSION)/$(ET_ZIP); \
	fi
	@unzip -o $(BUILD_DIR)/$(ET_ZIP) '$(ET_DIRNAME)/$(ET_CORE)' '$(ET_DIRNAME)/$(ET_CLI)' '$(ET_DIRNAME)/Packet.dll' '$(ET_DIRNAME)/wintun.dll' -d $(DIST)/ >/dev/null 2>&1 || \
		unzip -j -o $(BUILD_DIR)/$(ET_ZIP) '$(ET_DIRNAME)/$(ET_CORE)' '$(ET_DIRNAME)/$(ET_CLI)' '$(ET_DIRNAME)/Packet.dll' '$(ET_DIRNAME)/wintun.dll' -d $(DIST)/$(ET_DIRNAME) >/dev/null 2>&1 || true
	@chmod +x $(DIST)/$(ET_DIRNAME)/$(ET_CORE) $(DIST)/$(ET_DIRNAME)/$(ET_CLI) 2>/dev/null || true
	@cp -f packaging/THIRD_PARTY_NOTICES.md $(DIST)/ 2>/dev/null || true
	@if [ "$(PLATFORM)" = "windows" ]; then \
		PATH="$$HOME/.local/bin:$$PATH" upx --force --best --lzma $(DIST)/$(APP_BIN) 2>/dev/null || true; \
	fi
	@# easytier-core НЕ сжимаем UPX: это Rust-бинарник, и после UPX ломаются
	@# таблицы исключений/CFG — при запуске падает с 0xc0000005 (access violation).
	@echo "Готово: $(DIST)  (запуск: $(DIST)/$(APP_BIN))"

# Собирает и запускает приложение (для текущей платформы).
run: build
	$(DIST)/$(APP_BIN)

# Собирает и упаковывает дистрибутив в zip.
package: build
	@cd $(BUILD_DIR) && zip -qr PotatoNet-$(OS_NAME)-x64.zip dist-$(PLATFORM)
	@echo "Архив: $(BUILD_DIR)/PotatoNet-$(OS_NAME)-x64.zip"

# --- AppImage (только Linux) ---
$(APPIMAGE_TOOL):
	@mkdir -p $(BUILD_DIR)/tools
	@echo "Скачиваю appimagetool..."
	@curl -fL -o $@ https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage
	@chmod +x $@

ifeq ($(PLATFORM),windows)
appimage:
	@echo "AppImage собирается только для Linux (без PLATFORM=windows)"; false
else
appimage: build $(APPIMAGE_TOOL)
	@rm -rf $(BUILD_DIR)/PotatoNet.AppDir
	@mkdir -p $(BUILD_DIR)/PotatoNet.AppDir/usr/bin \
	           $(BUILD_DIR)/PotatoNet.AppDir/usr/share/applications \
	           $(BUILD_DIR)/PotatoNet.AppDir/usr/share/icons/hicolor/256x256/apps
	@cp -rT $(DIST) $(BUILD_DIR)/PotatoNet.AppDir/usr/bin
	@rm -f $(BUILD_DIR)/PotatoNet.AppDir/usr/bin/config.yml
	@install -m755 packaging/AppRun $(BUILD_DIR)/PotatoNet.AppDir/AppRun
	@install -m644 packaging/PotatoNet.desktop $(BUILD_DIR)/PotatoNet.AppDir/PotatoNet.desktop
	@install -m644 packaging/PotatoNet.desktop $(BUILD_DIR)/PotatoNet.AppDir/usr/share/applications/PotatoNet.desktop
	@install -m644 assets/icon.png $(BUILD_DIR)/PotatoNet.AppDir/PotatoNet.png
	@install -m644 assets/icon.png $(BUILD_DIR)/PotatoNet.AppDir/usr/share/icons/hicolor/256x256/apps/PotatoNet.png
	APPIMAGE_EXTRACT_AND_RUN=1 ARCH=x86_64 $(APPIMAGE_TOOL) \
		$(BUILD_DIR)/PotatoNet.AppDir $(BUILD_DIR)/PotatoNet-linux-x64.AppImage
	@echo "Готово: $(BUILD_DIR)/PotatoNet-linux-x64.AppImage"
endif

# --- Установщик Windows (NSIS) ---
# Предпочитаем системный makensis (в CI ставится из apt). Скачанный из
# Debian-пула бинарник собран под glibc 2.38+ и на Ubuntu 22.04 не запускается
# («GLIBC_2.38 not found»), поэтому он только как запасной вариант.
NSIS_VER     = 3.12-1
NSIS_DIR     = $(BUILD_DIR)/tools/nsis
NSIS_BIN     = $(NSIS_DIR)/usr/bin/makensis
SYS_MAKENSIS = $(shell command -v makensis 2>/dev/null)

ifeq ($(SYS_MAKENSIS),)
MAKENSIS    = $(NSIS_BIN)
NSISDIR_ENV = NSISDIR=$(abspath $(NSIS_DIR))/usr/share/nsis
else
MAKENSIS    = $(SYS_MAKENSIS)
NSISDIR_ENV =
endif

$(NSIS_BIN):
	@mkdir -p $(NSIS_DIR) $(BUILD_DIR)
	@echo "Скачиваю NSIS $(NSIS_VER)..."
	@curl -fL -o $(BUILD_DIR)/nsis-common.deb http://deb.debian.org/debian/pool/main/n/nsis/nsis-common_$(NSIS_VER)_all.deb
	@curl -fL -o $(BUILD_DIR)/nsis.deb http://deb.debian.org/debian/pool/main/n/nsis/nsis_$(NSIS_VER)_amd64.deb
	@cd $(NSIS_DIR) && ar x $(abspath $(BUILD_DIR)/nsis-common.deb) && for f in data.tar.*; do tar xf $$f; done && rm -f data.tar.* control.tar.* debian-binary
	@cd $(NSIS_DIR) && ar x $(abspath $(BUILD_DIR)/nsis.deb) && for f in data.tar.*; do tar xf $$f; done && rm -f data.tar.* control.tar.* debian-binary
	@rm -f $(BUILD_DIR)/nsis-common.deb $(BUILD_DIR)/nsis.deb
	@chmod +x $(NSIS_BIN)

ifeq ($(PLATFORM),windows)
installer: build $(MAKENSIS)
	@$(NSISDIR_ENV) $(MAKENSIS) -V2 -DAPP_VERSION=$(VERSION) packaging/installer.nsi
	@echo "Готово: $(BUILD_DIR)/PotatoNet-setup-x64.exe"
else
installer:
	@echo "Используйте: make installer PLATFORM=windows"; false
endif

test:
	$(GO) test $(WAILS_TAGS) ./...

# Удаляет только артефакты этого (PotatoNet) проекта, не трогая классический
# build/Linux и т.п. из ветки main (их использует systemd-служба PotatoNet).
clean:
	rm -rf $(BUILD_DIR)/dist-linux $(BUILD_DIR)/dist-win \
	       $(BUILD_DIR)/bin $(BUILD_DIR)/PotatoNet-linux-x64.zip $(BUILD_DIR)/PotatoNet-win-x64.zip \
	       $(BUILD_DIR)/PotatoNet.AppDir $(BUILD_DIR)/PotatoNet-linux-x64.AppImage \
	       $(BUILD_DIR)/PotatoNet-setup-x64.exe
