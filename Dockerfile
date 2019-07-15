FROM golang:1.12-alpine3.9 AS builder
ARG GOLANGCI_VERSION="1.16.0"
ARG GOLANGCI_SHASUM="5343fc3ffcbb9910925f4047ec3c9f2e9623dd56a72a17ac76fb2886abc0976b"

WORKDIR /app
RUN apk add --no-cache --virtual .go-deps git gcc musl-dev openssl pkgconf \
    && wget -q https://github.com/golangci/golangci-lint/releases/download/v$GOLANGCI_VERSION/golangci-lint-$GOLANGCI_VERSION-linux-amd64.tar.gz \
    && echo -n "$GOLANGCI_SHASUM  golangci-lint-$GOLANGCI_VERSION-linux-amd64.tar.gz" | sha256sum -c - \
    && tar xzf golangci-lint-$GOLANGCI_VERSION-linux-amd64.tar.gz \
    && rm golangci-lint-$GOLANGCI_VERSION-linux-amd64.tar.gz

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o crawler -ldflags '-w -s' main.go

# Run Golang CI Lint, with a whole suite of linters and code quality checks
RUN golangci-lint-$GOLANGCI_VERSION-linux-amd64/golangci-lint run
ENTRYPOINT ["/app/crawler"]

# Build the smallest image possible
FROM debian AS runner
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /app/crawler /crawler

ENTRYPOINT [ "/crawler" ]
