FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/lastro ./cmd/lastro

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/lastro /lastro
EXPOSE 8080
ENTRYPOINT ["/lastro"]
CMD ["serve"]
