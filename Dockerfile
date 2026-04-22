FROM golang:1.22

WORKDIR /app

# Copy go.mod first (important)
COPY go.mod ./
RUN go mod download

# Copy rest of code
COPY . .

# Build binary
RUN go build -o main .

EXPOSE 8080

CMD ["./main"]