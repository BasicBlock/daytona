FROM golang:1.26 AS build

WORKDIR /src
COPY apps/sandbox-controller/go.mod apps/sandbox-controller/go.sum ./apps/sandbox-controller/

WORKDIR /src/apps/sandbox-controller
RUN go mod download
COPY apps/sandbox-controller ./
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/sandbox-controller ./cmd/sandbox-controller
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/sandbox-api ./cmd/sandbox-api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/local-node-agent ./cmd/local-node-agent
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/toolbox-sidecar ./cmd/toolbox-sidecar

FROM registry.k8s.io/kubectl:v1.34.1 AS kubectl

FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates util-linux \
  && rm -rf /var/lib/apt/lists/*

COPY --from=build /out/sandbox-controller /sandbox-controller
COPY --from=build /out/sandbox-api /sandbox-api
COPY --from=build /out/local-node-agent /local-node-agent
COPY --from=build /out/toolbox-sidecar /toolbox-sidecar
COPY --from=kubectl /bin/kubectl /usr/local/bin/kubectl
USER 65532:65532
ENTRYPOINT ["/sandbox-controller"]
