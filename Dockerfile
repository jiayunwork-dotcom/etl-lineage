FROM golang:1.21-alpine
ENV GOTOOLCHAIN=local
ENV CGO_ENABLED=0
WORKDIR /app
COPY . .
RUN go build -o /app/bin/server .
EXPOSE 8080
CMD ["/app/bin/server", "serve"]
