init:
	go mod tidy
	git config core.hooksPath .githooks
	go install golang.org/x/lint/golint@latest
	terraform init

lint:
	go vet ./...
	golint -set_exit_status ./...

fmt:
	terraform fmt -recursive
	gofmt -s -w .

mock-gen:
	go generate ./...

clean:
	rm -rf dist/

lambda-env = GOOS=linux GOARCH=amd64 CGO_ENABLED=0

ldflags = "-s -w"

build-github-events:
	cd github-events/ingest && $(lambda-env) go build -ldflags=$(ldflags) -o ../../dist/github-events

build-github-app-tokens:
	cd github-app/tokens && $(lambda-env) go build -ldflags=$(ldflags) -o ../../dist/github-app-tokens

github-events-bundle: build-github-events
	cd dist && zip github-events.zip github-events

github-app-bundle: build-github-app-tokens
	cd dist && zip github-app.zip github-app-tokens

lambda-bundles: clean build-github-events build-github-app-tokens

create-github-app:
	cd github-app && go run main.go
