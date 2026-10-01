# ---- Stage 1: build binary Go ----
FROM golang:1.22-alpine AS build
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
# CGO_ENABLED=0 -> binary statis. -ldflags "-s -w" -> binary lebih kecil.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/backend .

# ---- Stage 2: runtime minimal (distroless, ~2MB) ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /app/backend /app/backend

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/backend"]
