FROM golang:1.23 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/dayzmods ./cmd/dayzmods

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /out/dayzmods /app/dayzmods
ENTRYPOINT ["/app/dayzmods", "run", "--config", "/app/config.json"]
