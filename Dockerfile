FROM golang:latest

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

COPY ./config/config.go ./config/
COPY ./routers_handlers/handlers.go ./routers_handlers/

RUN go mod download
RUN go build ./config/config.go
RUN go build ./routers_handlers/handlers.go

COPY *.go ./

RUN go build -o billing-app .

EXPOSE 8080:8080

CMD ["./billing-app"]