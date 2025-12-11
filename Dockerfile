FROM python:3.11-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y --no-install-recommends golang-go ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY . /app

RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir .

# Prebuild the Go proxy binary inside the image
RUN cd pygofastproxy/go && go build -o proxy *.go

ENV PY_BACKEND_TARGET=http://localhost:4000
ENV PY_BACKEND_PORT=8080

EXPOSE 8080

CMD ["pygofastproxy"]
