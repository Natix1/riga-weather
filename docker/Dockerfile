FROM golang:1.26.4

WORKDIR /app
COPY ../go.mod ../go.sum ./

RUN go mod download
COPY ../*.go ./

RUN go build -o app
CMD ["./app"]