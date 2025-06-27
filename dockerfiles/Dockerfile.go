FROM golang:1.22-alpine
WORKDIR /app
COPY go-services/ .
RUN go build -o app .
CMD ["./app"]
