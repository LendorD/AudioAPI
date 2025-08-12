# ------------------------
# 1. Stage: Build Go API
# ------------------------
FROM golang:1.24-alpine AS go-builder

WORKDIR /app

# Устанавливаем зависимости для Go
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o app

# ------------------------
# 2. Stage: Final image with Go + Python venv
# ------------------------
FROM python:3.10-slim AS final

WORKDIR /app

# Устанавливаем Go runtime (для запуска скомпилированного бинарника не нужно всё окружение Go)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libsndfile1 \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

# Создаём venv для Python
RUN python -m venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"

# Устанавливаем зависимости Python
COPY python-scripts/requirements.txt /python-scripts/
RUN pip install --no-cache-dir -r /python-scripts/requirements.txt

# Копируем бинарь Go
COPY --from=go-builder /app/app .

# Копируем Python-скрипты
COPY python-scripts/ ./python-scripts/

# Запуск Go API
CMD ["./app"]
