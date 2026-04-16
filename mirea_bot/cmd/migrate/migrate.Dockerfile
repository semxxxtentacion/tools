FROM golang:1.23 AS migrate

WORKDIR /app

COPY go.* ./

RUN go mod download

COPY pkg ./pkg
COPY internal ./internal
COPY cmd/migrate ./cmd/migrate

RUN go build -v -o /app/migrate ./cmd/migrate/main.go

CMD ["/app/migrate"]
