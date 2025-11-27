.PHONY: tailwind-watch
tailwind-watch:
	npm run watch

.PHONY: tailwind-build
tailwind-build:
	npm run build

.PHONY: templ-generate
templ-generate:
	templ generate

.PHONY: templ-watch
templ-watch:
	templ generate --watch
	
.PHONY: dev
dev:
	air

.PHONY: build
build:
	make tailwind-build
	make templ-generate
	go build -ldflags "-s -w -X main.Environment=production" -o ./rapua ./cmd/rapua/main.go

.PHONY: vet
vet:
	go vet ./...

.PHONY: staticcheck
staticcheck:
	staticcheck ./...

.PHONY: test
test:
	  go test -race -v -timeout 20s ./...
