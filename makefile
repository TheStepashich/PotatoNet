PLATFORM   ?= linux
ARCH       ?= amd64
NW_VERSION ?= 0.90.0
ET_VERSION ?= 2.4.5

GO        = go
NWJS_DIR  = ./nwjs
BUILD_DIR = ./build
SRC_DIR   = ./src

ifeq ($(PLATFORM),windows)
GOOS        = windows
NW_PLATFORM = win
NW_ARCHIVE  = nwjs-v$(NW_VERSION)-win-x64.zip
NW_DIRNAME  = nwjs-v$(NW_VERSION)-win-x64
NW_BIN      = nw.exe
APP_BIN     = pnet.exe
CORE_BIN    = pnet-core.exe
ET_DIRNAME  = easytier-windows-x86_64
ET_CORE     = easytier-core.exe
ET_CLI      = easytier-cli.exe
LDFLAGS     = -ldflags -H=windowsgui
else
GOOS        = linux
NW_PLATFORM = linux
NW_ARCHIVE  = nwjs-v$(NW_VERSION)-linux-x64.tar.gz
NW_DIRNAME  = nwjs-v$(NW_VERSION)-linux-x64
NW_BIN      = nw
APP_BIN     = pnet
CORE_BIN    = pnet-core
ET_DIRNAME  = easytier-linux-x86_64
ET_CORE     = easytier-core
ET_CLI      = easytier-cli
LDFLAGS     =
endif

ET_ZIP = $(ET_DIRNAME)-v$(ET_VERSION).zip

NW_RUNTIME = $(BUILD_DIR)/nw-runtime-$(NW_PLATFORM)
DIST       = $(BUILD_DIR)/dist-$(NW_PLATFORM)
ARCHIVE    = $(BUILD_DIR)/$(NW_ARCHIVE)

APPIMAGE_TOOL = $(BUILD_DIR)/tools/appimagetool
NSIS_VER      = 3.12-1
NSIS_DIR      = $(BUILD_DIR)/tools/nsis
MAKENSIS      = $(NSIS_DIR)/usr/bin/makensis

.PHONY: all build run package appimage installer nw-runtime test clean

all: build

# Самодостаточная папка build/dist-<platform>: рантайм NW.js + приложение + бэкенд.
# Пользователь запускает pnet(.exe); он поднимает pnet-core(.exe) с окном.
build: nw-runtime
	@mkdir -p $(BUILD_DIR)/.keep
	@[ -f $(DIST)/config.yml ] && cp -f $(DIST)/config.yml $(BUILD_DIR)/.keep/ || true
	@rm -rf $(DIST)
	@cp -rT $(NW_RUNTIME) $(DIST)
	@mv $(DIST)/$(NW_BIN) $(DIST)/$(APP_BIN)
	@cp -rT $(NWJS_DIR) $(DIST)
	@cp -rn $(BUILD_DIR)/.keep/. $(DIST)/ 2>/dev/null || true
	@mkdir -p $(DIST)/$(ET_DIRNAME)
	@if [ ! -f $(BUILD_DIR)/$(ET_ZIP) ]; then \
		echo "Скачиваю EasyTier $(ET_VERSION) ($(NW_PLATFORM))..."; \
		curl -fL -o $(BUILD_DIR)/$(ET_ZIP) \
			https://github.com/EasyTier/EasyTier/releases/download/v$(ET_VERSION)/$(ET_ZIP); \
	fi
	@unzip -j -o $(BUILD_DIR)/$(ET_ZIP) '$(ET_DIRNAME)/$(ET_CORE)' '$(ET_DIRNAME)/$(ET_CLI)' \
		-d $(DIST)/$(ET_DIRNAME) >/dev/null
	@chmod +x $(DIST)/$(ET_DIRNAME)/$(ET_CORE) $(DIST)/$(ET_DIRNAME)/$(ET_CLI) 2>/dev/null || true
	@cp -f packaging/THIRD_PARTY_NOTICES.md $(DIST)/ 2>/dev/null || true
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(ARCH) \
	$(GO) build $(LDFLAGS) -o $(DIST)/$(CORE_BIN) $(SRC_DIR)
	@rm -rf $(BUILD_DIR)/.keep
	@echo "Готово: $(DIST)  (запуск: $(DIST)/$(APP_BIN))"

# Скачивает портативный рантайм NW.js в build/nw-runtime-<platform> (один раз).
nw-runtime:
	@if [ ! -f $(NW_RUNTIME)/$(NW_BIN) ]; then \
		echo "Скачиваю NW.js $(NW_VERSION) ($(NW_PLATFORM))..."; \
		mkdir -p $(NW_RUNTIME) $(BUILD_DIR)/nwtmp; \
		curl -fL -o $(ARCHIVE) https://dl.nwjs.io/v$(NW_VERSION)/$(NW_ARCHIVE); \
		case "$(NW_ARCHIVE)" in \
			*.zip) unzip -q -o $(ARCHIVE) -d $(BUILD_DIR)/nwtmp ;; \
			*.tar.gz) tar xzf $(ARCHIVE) -C $(BUILD_DIR)/nwtmp ;; \
		esac; \
		cp -rT $(BUILD_DIR)/nwtmp/$(NW_DIRNAME) $(NW_RUNTIME); \
		rm -rf $(BUILD_DIR)/nwtmp $(ARCHIVE); \
	fi

# Собирает и запускает окно приложения (для текущей платформы).
run: build
	$(DIST)/$(APP_BIN)

# Собирает и упаковывает дистрибутив в zip.
package: build
	@cd $(BUILD_DIR) && zip -qr pnet-$(NW_PLATFORM)-x64.zip dist-$(NW_PLATFORM)
	@echo "Архив: $(BUILD_DIR)/pnet-$(NW_PLATFORM)-x64.zip"

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
	@rm -rf $(BUILD_DIR)/pnet.AppDir
	@mkdir -p $(BUILD_DIR)/pnet.AppDir/usr/bin \
	           $(BUILD_DIR)/pnet.AppDir/usr/share/applications \
	           $(BUILD_DIR)/pnet.AppDir/usr/share/icons/hicolor/256x256/apps
	@cp -rT $(DIST) $(BUILD_DIR)/pnet.AppDir/usr/bin
	@rm -f $(BUILD_DIR)/pnet.AppDir/usr/bin/config.yml
	@install -m755 packaging/AppRun $(BUILD_DIR)/pnet.AppDir/AppRun
	@install -m644 packaging/pnet.desktop $(BUILD_DIR)/pnet.AppDir/pnet.desktop
	@install -m644 packaging/pnet.desktop $(BUILD_DIR)/pnet.AppDir/usr/share/applications/pnet.desktop
	@install -m644 assets/icon.png $(BUILD_DIR)/pnet.AppDir/pnet.png
	@install -m644 assets/icon.png $(BUILD_DIR)/pnet.AppDir/usr/share/icons/hicolor/256x256/apps/pnet.png
	APPIMAGE_EXTRACT_AND_RUN=1 ARCH=x86_64 $(APPIMAGE_TOOL) \
		$(BUILD_DIR)/pnet.AppDir $(BUILD_DIR)/pnet-linux-x64.AppImage
	@echo "Готово: $(BUILD_DIR)/pnet-linux-x64.AppImage"
endif

# --- Установщик Windows (NSIS, makensis из deb-пакетов Debian) ---
$(MAKENSIS):
	@mkdir -p $(NSIS_DIR) $(BUILD_DIR)
	@echo "Скачиваю NSIS $(NSIS_VER)..."
	@curl -fL -o $(BUILD_DIR)/nsis-common.deb http://deb.debian.org/debian/pool/main/n/nsis/nsis-common_$(NSIS_VER)_all.deb
	@curl -fL -o $(BUILD_DIR)/nsis.deb http://deb.debian.org/debian/pool/main/n/nsis/nsis_$(NSIS_VER)_amd64.deb
	@cd $(NSIS_DIR) && ar x $(abspath $(BUILD_DIR)/nsis-common.deb) && for f in data.tar.*; do tar xf $$f; done && rm -f data.tar.* control.tar.* debian-binary
	@cd $(NSIS_DIR) && ar x $(abspath $(BUILD_DIR)/nsis.deb) && for f in data.tar.*; do tar xf $$f; done && rm -f data.tar.* control.tar.* debian-binary
	@rm -f $(BUILD_DIR)/nsis-common.deb $(BUILD_DIR)/nsis.deb
	@chmod +x $(MAKENSIS)

ifeq ($(PLATFORM),windows)
installer: build $(MAKENSIS)
	@NSISDIR=$(abspath $(NSIS_DIR))/usr/share/nsis $(MAKENSIS) -V2 packaging/installer.nsi
	@echo "Готово: $(BUILD_DIR)/pnet-setup-x64.exe"
else
installer:
	@echo "Используйте: make installer PLATFORM=windows"; false
endif

test:
	$(GO) test ./...

# Удаляет только артефакты этого (potatonet) проекта, не трогая классический
# build/Linux и т.п. из ветки main (их использует systemd-служба pnet).
clean:
	rm -rf $(BUILD_DIR)/dist-linux $(BUILD_DIR)/dist-win \
	       $(BUILD_DIR)/nw-runtime-linux $(BUILD_DIR)/nw-runtime-win \
	       $(BUILD_DIR)/nwjs $(BUILD_DIR)/pnet-linux-x64.zip $(BUILD_DIR)/pnet-win-x64.zip \
	       $(BUILD_DIR)/pnet.AppDir $(BUILD_DIR)/pnet-linux-x64.AppImage \
	       $(BUILD_DIR)/pnet-setup-x64.exe
