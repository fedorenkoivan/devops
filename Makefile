.PHONY: lint lint-go lint-docker lint-shell lint-yaml test coverage

lint: lint-go lint-docker lint-shell lint-yaml

lint-go:
	golangci-lint run ./...

lint-docker:
	hadolint Dockerfile

lint-shell:
	find . -name '*.sh' -not -path './.git/*' | xargs shellcheck

lint-yaml:
	yamllint .github/workflows/

test:
	go test -race ./...

coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"
