# Makefile para o projeto Eliot

# Variáveis
BINARY_NAME=eliot
MAIN_PATH=./cmd/eliot
BUILD_DIR=./bin
DOCKER_IMAGE=fabianoflorentino/eliot
DOCKER_TAG=v0.0.2
GO_VERSION=1.24

# Cores para output
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[1;33m
NC=\033[0m # No Color

.PHONY: help
help: ## Mostra esta mensagem de ajuda
	@echo "Comandos disponíveis:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'

.PHONY: clean
clean: ## Remove arquivos de build e cache
	@echo "$(YELLOW)Limpando arquivos de build...$(NC)"
	rm -rf $(BUILD_DIR)
	go clean -cache
	go clean -modcache
	@echo "$(GREEN)Limpeza concluída!$(NC)"

.PHONY: deps
deps: ## Baixa e atualiza dependências
	@echo "$(YELLOW)Baixando dependências...$(NC)"
	go mod download
	go mod tidy
	@echo "$(GREEN)Dependências atualizadas!$(NC)"

.PHONY: build
build: ## Compila o projeto
	@echo "$(YELLOW)Compilando $(BINARY_NAME)...$(NC)"
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "$(GREEN)Build concluído: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

.PHONY: build-local
build-local: ## Compila o projeto para o sistema local
	@echo "$(YELLOW)Compilando $(BINARY_NAME) para sistema local...$(NC)"
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "$(GREEN)Build local concluído: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

.PHONY: run
run: ## Executa o projeto localmente
	@echo "$(YELLOW)Executando $(BINARY_NAME)...$(NC)"
	HTTP_ADDR=:8080 REDIS_ADDR=localhost:6379 go run $(MAIN_PATH)

.PHONY: run-local
run-local: ## Executa o projeto localmente com Redis via Docker
	@echo "$(YELLOW)Iniciando Redis temporário e executando $(BINARY_NAME)...$(NC)"
	@docker run -d --name eliot-redis-dev -p 6379:6379 --rm redis:alpine3.22 > /dev/null 2>&1 || true
	@sleep 2
	@HTTP_ADDR=:8080 REDIS_ADDR=localhost:6379 go run $(MAIN_PATH) || true
	@echo "$(YELLOW)Parando Redis temporário...$(NC)"
	@docker stop eliot-redis-dev > /dev/null 2>&1 || true

.PHONY: run-dev
run-dev: ## Executa o projeto em modo desenvolvimento com auto-reload
	@echo "$(YELLOW)Executando $(BINARY_NAME) em modo desenvolvimento...$(NC)"
	@if command -v air > /dev/null 2>&1; then \
		HTTP_ADDR=:8080 REDIS_ADDR=localhost:6379 air; \
	elif [ -f "$$HOME/go/bin/air" ]; then \
		HTTP_ADDR=:8080 REDIS_ADDR=localhost:6379 $$HOME/go/bin/air; \
	elif [ -f "$$(go env GOROOT)/../bin/air" ]; then \
		HTTP_ADDR=:8080 REDIS_ADDR=localhost:6379 $$(go env GOROOT)/../bin/air; \
	else \
		echo "$(RED)Air não encontrado. Instale com: go install github.com/air-verse/air@latest$(NC)"; \
		echo "$(YELLOW)Executando sem auto-reload...$(NC)"; \
		HTTP_ADDR=:8080 REDIS_ADDR=localhost:6379 go run $(MAIN_PATH); \
	fi

.PHONY: test
test: ## Executa todos os testes
	@echo "$(YELLOW)Executando testes...$(NC)"
	go test -v ./...

.PHONY: test-coverage
test-coverage: ## Executa testes com cobertura
	@echo "$(YELLOW)Executando testes com cobertura...$(NC)"
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Relatório de cobertura gerado: coverage.html$(NC)"

.PHONY: test-race
test-race: ## Executa testes com detecção de race conditions
	@echo "$(YELLOW)Executando testes com detecção de race conditions...$(NC)"
	go test -race -v ./...

.PHONY: lint
lint: ## Executa linting no código
	@echo "$(YELLOW)Executando linting...$(NC)"
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run; \
	elif [ -f "$$HOME/go/bin/golangci-lint" ]; then \
		$$HOME/go/bin/golangci-lint run; \
	elif [ -f "$$(go env GOROOT)/../bin/golangci-lint" ]; then \
		$$(go env GOROOT)/../bin/golangci-lint run; \
	else \
		echo "$(RED)golangci-lint não encontrado. Instale com: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(NC)"; \
		go vet ./...; \
	fi

.PHONY: fmt
fmt: ## Formata o código
	@echo "$(YELLOW)Formatando código...$(NC)"
	go fmt ./...
	@if command -v goimports > /dev/null 2>&1; then \
		goimports -w .; \
	elif [ -f "$$HOME/go/bin/goimports" ]; then \
		$$HOME/go/bin/goimports -w .; \
	elif [ -f "$$(go env GOROOT)/../bin/goimports" ]; then \
		$$(go env GOROOT)/../bin/goimports -w .; \
	else \
		echo "$(YELLOW)goimports não encontrado. Instale com: go install golang.org/x/tools/cmd/goimports@latest$(NC)"; \
	fi

.PHONY: vet
vet: ## Executa go vet
	@echo "$(YELLOW)Executando go vet...$(NC)"
	go vet ./...

.PHONY: check
check: fmt vet lint test ## Executa todas as verificações (fmt, vet, lint, test)

# Remove todas as imagens do projeto, incluindo sem tags
.PHONY: docker-image-remove
docker-image-remove: ## Remove imagens Docker do projeto (com e sem tags)
	@echo "$(YELLOW)Removendo imagens Docker do projeto...$(NC)"
	@docker images --filter=reference='$(DOCKER_IMAGE)*' -q | xargs -r docker rmi -f
	@docker images -f "dangling=true" -q | xargs -r docker rmi -f
	@echo "$(GREEN)Imagens removidas!$(NC)"

.PHONY: docker-build
docker-build: ## Constrói a imagem Docker
	@echo "$(YELLOW)Construindo imagem Docker...$(NC)"
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "$(GREEN)Imagem Docker construída: $(DOCKER_IMAGE):$(DOCKER_TAG)$(NC)"

.PHONY: docker-push
docker-push: docker-build ## Envia a imagem Docker para o repositório
	@echo "$(YELLOW)Enviando imagem Docker para o repositório...$(NC)"
	@docker push $(DOCKER_IMAGE):$(DOCKER_TAG) || { echo "$(RED)Falha ao enviar imagem Docker!$(NC)"; exit 1; }
	echo "$(GREEN)Imagem Docker enviada: $(DOCKER_IMAGE):$(DOCKER_TAG)$(NC)"

.PHONY: docker-run
docker-run: ## Executa o container Docker
	@echo "$(YELLOW)Executando container Docker...$(NC)"
	docker run --rm -p 8080:8080 \
		-e HTTP_ADDR=:8080 \
		-e REDIS_ADDR=localhost:6379 \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: docker-compose-up
docker-compose-up: ## Sobe todos os serviços com docker-compose
	@echo "$(YELLOW)Subindo serviços com docker-compose...$(NC)"
	docker compose up -d
	@echo "$(GREEN)Serviços iniciados!$(NC)"

.PHONY: docker-compose-down
docker-compose-down: ## Para todos os serviços do docker-compose
	@echo "$(YELLOW)Parando serviços do docker-compose...$(NC)"
	docker compose down
	@echo "$(GREEN)Serviços parados!$(NC)"

.PHONY: docker-compose-logs
docker-compose-logs: ## Mostra logs dos serviços
	docker compose logs -f

.PHONY: docker-compose-restart
docker-compose-restart: docker-compose-down docker-build docker-compose-up ## Reinicia todos os serviços

.PHONY: docker-compose-retest
docker-compose-retest: delete-partial-results docker-compose-down docker-image-remove docker-build docker-compose-up k6-test show-partial-results ## Reinicia todos os serviços e executa os testes de carga

.PHONY: delete-partial-results
delete-partial-results: ## Remove resultados parciais
	@echo "$(YELLOW)Removendo resultados parciais...$(NC)"
	@rm -f partial-results.json
	@echo "$(GREEN)Resultados parciais removidos!$(NC)"

.PHONY: show-partial-results
show-partial-results: ## Mostra resultados parciais
	@echo "$(YELLOW)Resultados parciais...$(NC)"
	@if [ -f partial-results.json ]; then \
		jq . partial-results.json; \
	else \
		echo "$(RED)Resultados parciais não encontrados!$(NC)"; \
	fi

.PHONY: redis-start
redis-start: ## Inicia Redis localmente
	@echo "$(YELLOW)Iniciando Redis...$(NC)"
	@if command -v redis-server > /dev/null; then \
		redis-server --daemonize yes --port 6379; \
		echo "$(GREEN)Redis iniciado na porta 6379$(NC)"; \
	else \
		echo "$(RED)Redis não encontrado. Instale o Redis ou use docker-compose$(NC)"; \
	fi

.PHONY: redis-stop
redis-stop: ## Para Redis local
	@echo "$(YELLOW)Parando Redis...$(NC)"
	@if command -v redis-cli > /dev/null; then \
		redis-cli shutdown; \
		echo "$(GREEN)Redis parado$(NC)"; \
	else \
		echo "$(RED)Redis CLI não encontrado$(NC)"; \
	fi

.PHONY: k6-test
k6-test: ## Executa testes de carga com k6
	@echo "$(YELLOW)Executando testes k6...$(NC)"
	@if command -v k6 > /dev/null; then \
		MAX_REQUESTS=550 k6 run ./k6/rinha.js; \
	else \
		echo "$(RED)k6 não encontrado. Instale o k6 para executar testes de carga$(NC)"; \
	fi

.PHONY: install-tools
install-tools: ## Instala ferramentas de desenvolvimento
	@echo "$(YELLOW)Instalando ferramentas de desenvolvimento...$(NC)"
	go install github.com/air-verse/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	@echo "$(GREEN)Ferramentas instaladas!$(NC)"
	@echo "$(YELLOW)Verifique se $$HOME/go/bin está no seu PATH$(NC)"

.PHONY: check-tools
check-tools: ## Verifica se as ferramentas estão instaladas
	@echo "$(YELLOW)Verificando ferramentas de desenvolvimento...$(NC)"
	@echo -n "Air: "; \
	if command -v air > /dev/null 2>&1 || [ -f "$$HOME/go/bin/air" ] || [ -f "$$(go env GOROOT)/../bin/air" ]; then \
		echo "$(GREEN)✓$(NC)"; \
	else \
		echo "$(RED)✗$(NC)"; \
	fi
	@echo -n "golangci-lint: "; \
	if command -v golangci-lint > /dev/null 2>&1 || [ -f "$$HOME/go/bin/golangci-lint" ] || [ -f "$$(go env GOROOT)/../bin/golangci-lint" ]; then \
		echo "$(GREEN)✓$(NC)"; \
	else \
		echo "$(RED)✗$(NC)"; \
	fi
	@echo -n "goimports: "; \
	if command -v goimports > /dev/null 2>&1 || [ -f "$$HOME/go/bin/goimports" ] || [ -f "$$(go env GOROOT)/../bin/goimports" ]; then \
		echo "$(GREEN)✓$(NC)"; \
	else \
		echo "$(RED)✗$(NC)"; \
	fi
	@echo -n "govulncheck: "; \
	if command -v govulncheck > /dev/null 2>&1 || [ -f "$$HOME/go/bin/govulncheck" ] || [ -f "$$(go env GOROOT)/../bin/govulncheck" ]; then \
		echo "$(GREEN)✓$(NC)"; \
	else \
		echo "$(RED)✗$(NC)"; \
	fi

.PHONY: mod-update
mod-update: ## Atualiza todas as dependências para versões mais recentes
	@echo "$(YELLOW)Atualizando dependências...$(NC)"
	go get -u ./...
	go mod tidy
	@echo "$(GREEN)Dependências atualizadas!$(NC)"

.PHONY: security-check
security-check: ## Verifica vulnerabilidades de segurança
	@echo "$(YELLOW)Verificando vulnerabilidades...$(NC)"
	@if command -v govulncheck > /dev/null; then \
		govulncheck ./...; \
	else \
		echo "$(YELLOW)govulncheck não encontrado. Instale com: go install golang.org/x/vuln/cmd/govulncheck@latest$(NC)"; \
		go list -json -m all | grep -v "indirect" || true; \
	fi

.PHONY: benchmark
benchmark: ## Executa benchmarks
	@echo "$(YELLOW)Executando benchmarks...$(NC)"
	go test -bench=. -benchmem ./...

.PHONY: profile
profile: ## Gera profile de CPU
	@echo "$(YELLOW)Gerando profile de CPU...$(NC)"
	go test -cpuprofile=cpu.prof -bench=. ./...
	@echo "$(GREEN)Profile gerado: cpu.prof$(NC)"
	@echo "$(YELLOW)Para visualizar: go tool pprof cpu.prof$(NC)"

.PHONY: health-check
health-check: ## Verifica saúde da aplicação
	@echo "$(YELLOW)Verificando saúde da aplicação...$(NC)"
	@curl -f http://localhost:8080/health || echo "$(RED)Aplicação não está respondendo$(NC)"

.PHONY: docs
docs: ## Gera documentação
	@echo "$(YELLOW)Gerando documentação...$(NC)"
	@if command -v godoc > /dev/null; then \
		echo "$(GREEN)Documentação disponível em: http://localhost:6060/pkg/github.com/fabianoflorentino/eliot/$(NC)"; \
		godoc -http=:6060; \
	else \
		echo "$(RED)godoc não encontrado. Instale com: go install golang.org/x/tools/cmd/godoc@latest$(NC)"; \
	fi

.PHONY: release
release: clean check build docker-build ## Prepara release (clean, check, build, docker-build)
	@echo "$(GREEN)Release preparado com sucesso!$(NC)"

# Target padrão
.DEFAULT_GOAL := help

.PHONY: payment-processor-up
payment-processor-up: ## Inicia os serviços payment-processor via docker-compose
	@echo "Iniciando payment-processor..."
	cd payment-processor && docker compose up -d && docker compose logs -f
	@echo "payment-processor iniciado!"

.PHONY: payment-processor-down
payment-processor-down: ## Para os serviços payment-processor via docker-compose
	@echo "Parando payment-processor..."
	cd payment-processor && docker compose down --volumes --remove-orphans
	@echo "payment-processor parado!"

