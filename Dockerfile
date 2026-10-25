FROM golang:1.22-alpine AS builder
WORKDIR /app

# Fetch tailwind binary once
RUN wget -qO /usr/local/bin/tailwindcss https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64 \
    && chmod +x /usr/local/bin/tailwindcss

COPY go.* ./
RUN go mod download

COPY . .

RUN tailwindcss -i input.css -o static/style.css --minify
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o portfolio .

FROM gcr.io/distroless/static
WORKDIR /app
COPY --from=builder /app/portfolio /app/portfolio
COPY --from=builder /app/static /app/static
COPY --from=builder /app/templates /app/templates
EXPOSE 8080
CMD ["/app/portfolio"]