FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/lastro ./cmd/lastro

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/lastro /lastro
# O usuário nonroot só grava aqui: é onde ficam lastro.db e o banco do sandbox.
WORKDIR /home/nonroot
EXPOSE 8080
ENTRYPOINT ["/lastro"]
CMD ["serve"]
