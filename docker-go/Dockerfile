FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod tidy

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /synergazing_backend .

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /synergazing_backend .

COPY .env .
COPY Api.yml .
COPY storage ./storage

EXPOSE 3002

CMD ["./synergazing_backend"]