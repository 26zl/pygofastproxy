FROM python:3.11-slim

ENV DEBIAN_FRONTEND=noninteractive

# Install Go 1.25.5
RUN apt-get update && \
    apt-get install -y --no-install-recommends wget ca-certificates && \
    wget -q https://go.dev/dl/go1.25.5.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.25.5.linux-amd64.tar.gz && \
    rm go1.25.5.linux-amd64.tar.gz && \
    apt-get remove -y wget && \
    apt-get autoremove -y && \
    rm -rf /var/lib/apt/lists/*

ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /app

COPY . /app

RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir .

# Prebuild the Go proxy binary inside the image
RUN cd pygofastproxy/go && go build -o proxy .

ENV PY_BACKEND_TARGET=http://localhost:4000
ENV PY_BACKEND_PORT=8080

EXPOSE 8080

CMD ["python", "-m", "pygofastproxy"]
