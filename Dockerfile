FROM golang:1.27-alpine AS builder

WORKDIR /src

COPY go.mod .
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/cow-collector ./cmd/cow-collector

FROM alpine:3.22

WORKDIR /app

COPY --from=builder /out/cow-collector /usr/local/bin/cow-collector

EXPOSE 8081

ENTRYPOINT ["/usr/local/bin/cow-collector"]
