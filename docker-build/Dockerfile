# Debian 13 slim; refresh this digest deliberately when updating the base image.
FROM debian:trixie-slim@sha256:a72b4433de1cf7d6bae0c4cee74d0547f58601a643b3375f070c51600d63b6b2

# Preserve the compiler version while updating its operating-system libraries.
COPY mosmlfiles/mosml_2.10.1-0ubuntu2_amd64.deb /tmp/mosml.deb
RUN echo 'beb14c20b1ba8e888df908a56b6c03be4e18012b98f8a0c479f92d1fd1224913  /tmp/mosml.deb' | sha256sum -c - \
 && apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get upgrade -y \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends /tmp/mosml.deb \
 && rm -rf /var/lib/apt/lists/* /tmp/mosml.deb \
 && useradd --uid 4242 --create-home --user-group --shell /usr/sbin/nologin mosmluser

USER 4242:4242
WORKDIR /home/mosmluser
ENTRYPOINT ["/usr/bin/mosml", "-P", "full", "-liberal"]
