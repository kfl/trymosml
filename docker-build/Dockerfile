FROM debian:wheezy
MAINTAINER Ken Friis Larsen <ken@friislarsen.net>

RUN apt-get update && apt-get install -y \
    libgmp10

ADD ./mosmlfiles /mosmlfiles

RUN dpkg -i /mosmlfiles/mosml_2.10.1-0ubuntu2_amd64.deb

# Try Moscow ML specific stuff

RUN apt-get install -y adduser && \
    adduser --uid 4242 --disabled-login --gecos "Moscow ML user" \
    mosmluser

WORKDIR /home/mosmluser

ENTRYPOINT ulimit -p 5 ; su mosmluser -c '/usr/bin/mosml -P full -liberal'


