FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
# O painel compilado entra antes do go build, que o embute (web/embed.go).
COPY --from=web /src/web/dist/painel ./web/dist/painel
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/vintem ./cmd/vintem

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/vintem /vintem
# O usuário nonroot só grava aqui: é onde ficam vintem.db e o banco do sandbox.
WORKDIR /home/nonroot
EXPOSE 8080
ENTRYPOINT ["/vintem"]
CMD ["serve"]
