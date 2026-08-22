version := env("VERSION", "dev")
revision := `git rev-parse HEAD 2>/dev/null || echo unknown`

# matches the realm in .docker/keycloak, whose redirect uri is pinned to localhost:8080
export BASE_URL := env("BASE_URL", "http://localhost:8080")
export OIDC_ISSUER := env("OIDC_ISSUER", "http://localhost:8081/realms/simple-fileupload")
export OIDC_CLIENT_ID := env("OIDC_CLIENT_ID", "simple-fileupload")
export OIDC_CLIENT_SECRET := env("OIDC_CLIENT_SECRET", "dev-secret-not-for-production")

default:
    @just --list

[group('infra')]
infra-up:
    docker compose up -d

[group('infra')]
infra-stop:
    docker compose down

[group('infra')]
infra-down:
    docker compose down -v

[group('infra')]
infra-logs *args:
    docker compose logs -f {{ args }}

[group('infra')]
infra-ps:
    docker compose ps

[group('dev')]
run:
    go run .

# waits for keycloak to be healthy, oidc discovery happens before the server listens
[group('dev')]
dev:
    docker compose up -d --wait
    go run .

[group('dev')]
build:
    go build -ldflags "-X main.version={{ version }} -X main.revision={{ revision }}" -o tmp/simple-fileupload .

[group('dev')]
image tag=("simple-fileupload:" + version):
    docker build --build-arg VERSION={{ version }} --build-arg REVISION={{ revision }} -t {{ tag }} .

[group('dev')]
clean:
    rm -rf tmp

[group('check')]
test *args:
    go test -race {{ if args == "" { "./..." } else { args } }}

[group('check')]
vet:
    go vet ./...

[group('check')]
lint *args:
    golangci-lint run {{ args }}

[group('check')]
fmt:
    gofmt -l -w .

[group('check')]
ci: vet test lint
