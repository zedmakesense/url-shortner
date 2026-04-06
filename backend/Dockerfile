FROM golang:1.26 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux \
  go build -o /bin/app ./cmd/server/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /bin/app /bin/app

EXPOSE 8080

CMD ["/bin/app"]
