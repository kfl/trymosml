FROM --platform=$BUILDPLATFORM golang:1.27.4-trixie AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY webshell/go.mod webshell/go.sum ./
RUN go mod download
COPY webshell/*.go ./
COPY webshell/cmd ./cmd
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/webshell . \
 && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/smoke ./cmd/smoke \
 && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go test -c -o /out/session-tests .

FROM docker:29.9.3-dind
RUN apk add --no-cache caddy python3
ENV DOCKER_HOST=unix:///var/run/docker.sock \
    DOCKER_TLS_CERTDIR="" \
    TRYMOSML_DOCKER=/usr/local/bin/docker \
    TRYMOSML_REPL_IMAGE=trymosml/repl:dev \
    TRYMOSML_SITE=http://:80
COPY --from=build /out /opt/trymosml/bin
COPY --from=repl_archive /repl.tar /opt/trymosml/repl.tar
COPY deploy/Caddyfile deploy/webshell.json deploy/repl.py deploy/repl_image.py deploy/repl-image.json /opt/trymosml/deploy/
COPY web /opt/trymosml/web
COPY dev /opt/trymosml/dev
RUN chmod +x /opt/trymosml/deploy/repl.py
# The existing Docker integration tests resolve ../deploy/repl.py from here.
WORKDIR /opt/trymosml/webshell
EXPOSE 80
ENTRYPOINT ["docker-init", "--", "python3", "/opt/trymosml/dev/start.py"]
CMD []
