FROM golang:1.13 as base

WORKDIR /app

COPY . .

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main cmd/receiver/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=base /app/main ./main

EXPOSE 8080
EXPOSE 9102

CMD ["./main"]
