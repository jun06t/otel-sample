FROM golang:1.26-alpine AS build
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /usr/local/bin/app .

FROM gcr.io/distroless/static-debian12
COPY --from=build /usr/local/bin/app /app
ENTRYPOINT ["/app"]
