# Stage 1: Build Go binary
FROM golang:1.25.8-bookworm AS go-builder

WORKDIR /build
COPY pygofastproxy/go/ .
RUN go build -trimpath -ldflags="-s -w" -o proxy .

# Stage 2: Runtime
FROM python:3.11-slim

WORKDIR /app

COPY . /app
COPY --from=go-builder /build/proxy /app/pygofastproxy/go/proxy

RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir . && \
    chmod 755 /app/pygofastproxy/go/proxy && \
    useradd -r -s /bin/false proxyuser

USER proxyuser

ENV PY_BACKEND_TARGET=http://localhost:4000
ENV PY_BACKEND_PORT=8080

EXPOSE 8080

CMD ["python", "-m", "pygofastproxy"]
