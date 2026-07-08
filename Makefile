BINARY := crunchyroll-downloader

GO ?= go
DIST := dist

.PHONY: help
help:
	@echo "╔════════════════════════════════════════════════════╗"
	@echo "║  Crunchyroll Downloader                           ║"
	@echo "╠════════════════════════════════════════════════════╣"
	@echo "║  make build        Compila o binário               ║"
	@echo "║  make run          Executa (exige --url ou --file)  ║"
	@echo "║  make deps         Verifica dependências            ║"
	@echo "║  make clean        Remove artefatos de build        ║"
	@echo "║  make help         Mostra esta ajuda                ║"
	@echo "╚════════════════════════════════════════════════════╝"

.PHONY: build
build:
	$(GO) mod tidy
	@mkdir -p $(DIST)
	$(GO) build -o $(DIST)/$(BINARY) .

.PHONY: run
run: build
	$(DIST)/$(BINARY) $(ARGS)

.PHONY: deps
deps:
	@echo "◆ Verificando dependências Go..."
	@$(GO) version >/dev/null 2>&1 || { echo "✗ Go não encontrado. Instale Go 1.25+"; exit 1; }
	@echo "  ✓ Go: $$($(GO) version)"
	@$(GO) mod download 2>&1 | tail -1
	@echo ""
	@echo "◆ Verificando dependências do sistema..."
	@for cmd in ffmpeg ffprobe; do \
		if command -v $$cmd >/dev/null 2>&1; then \
			echo "  ✓ $$cmd: $$($$cmd -version 2>&1 | head -1)"; \
		else \
			echo "  ✗ $$cmd não encontrado. Instale com seu gerenciador de pacotes."; \
		fi; \
	done
	@echo ""
	@echo "◆ Todas as verificações concluídas."

.PHONY: clean
clean:
	@rm -rf $(DIST)
	@echo "  ✓ Artefatos removidos: $(DIST)/"
