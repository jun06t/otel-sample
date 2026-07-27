FROM golang:1.25-alpine AS build
WORKDIR /workspace
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /usr/local/bin/api .

FROM gcr.io/distroless/static-debian12
COPY --from=build /usr/local/bin/api /api
EXPOSE 8000
ENTRYPOINT ["/api"]
