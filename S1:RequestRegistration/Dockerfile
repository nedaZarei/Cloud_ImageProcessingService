FROM golang:1.23.2-alpine
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o requestregister

EXPOSE 5000

ENTRYPOINT ["/app/requestregister"]