# flag_storage_product/Dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata
RUN mkdir -p /app/uploads/products
COPY --from=builder /server /server
COPY migrations /migrations
EXPOSE 8083
CMD ["/server"]
