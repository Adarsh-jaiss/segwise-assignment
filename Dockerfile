FROM golang:1.21-alpine as build

WORKDIR /app

RUN adduser -D -u 1001 nonroot

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o main

FROM scratch

COPY .env .env

COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /app/main main
COPY --from=build /app/migrations ./migrations

USER nonroot

EXPOSE 8080

CMD ["./main"]
