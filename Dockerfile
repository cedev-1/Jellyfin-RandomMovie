FROM golang:1.24-alpine

WORKDIR /app

COPY . .

RUN go mod download && go build -o jellyfin-random .

RUN mkdir -p /app/data

EXPOSE 8080

CMD ["./jellyfin-random"]
