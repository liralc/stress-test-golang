# syntax=docker/dockerfile:1

# ---- build ----
FROM golang:1.24-alpine AS build
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/stresstest .

# ---- runtime ----
FROM scratch
# Certificados raiz para testar URLs https://
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/stresstest /stresstest
USER 65534:65534
ENTRYPOINT ["/stresstest"]
