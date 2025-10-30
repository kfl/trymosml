# Self contained Go build file that will download and install (locally) the correct
# version of Go, and build our programs. Go does not need to be installed on the
# system (and if it already is, it will be ignored).

# To manually invoke the locally installed Go, use ./go

webshell: go-workspace go $(wildcard *.go) go.mod
	./go mod tidy
	./go build

# Go installation config.
GO_VER=1.25.3
SYSTEM_NAME:=$(shell uname -s | tr '[:upper:]' '[:lower:]')
SYSTEM_ARCH:=$(shell uname -m)
# Map system architecture to Go architecture
ARCH_x86_64:=amd64
ARCH_aarch64:=arm64
ARCH_arm64:=arm64
ARCH_i386:=386
ARCH_i686:=386
GO_ARCH:=$(or $(ARCH_$(SYSTEM_ARCH)),$(SYSTEM_ARCH))
GO_VERSION:=$(GO_VER).$(SYSTEM_NAME)-$(GO_ARCH)
GO_DOWNLOAD_URL=https://golang.org/dl/go$(GO_VERSION).tar.gz


# Create local go workspace
go-workspace:
	mkdir go-workspace

# Setup ./go wrapper to use local GOPATH/GOROOT.
# Need to set PATH for gofmt.
go: _go-v$(GO_VERSION)
	@echo '#!/bin/sh' > $@
	@echo export PATH=$(abspath _go-v$(GO_VERSION)/bin):$(PATH) >> $@
	@echo mkdir -p $(abspath go-workspace) >> $@
	@echo GOPATH=$(abspath go-workspace) GOROOT=$(abspath _go-v$(GO_VERSION)) $(abspath _go-v$(GO_VERSION)/bin/go) \$$@ >> $@
	chmod +x $@
	@echo 'Created ./$@ wrapper'

# Download and unpack Go distribution.
_go-v$(GO_VERSION):
	mkdir $@
	@echo Downloading and unpacking Go $(GO_VERSION) to $@
	wget -O - $(GO_DOWNLOAD_URL) | tar xzf - --strip-components=1 -C $@


# Clean up binaries, local Go workspace and local go installation
.PHONY: clean superclean

clean:
	rm -rf webshell

superclean: clean
	-chmod -R u+w go-workspace 2>/dev/null || true
	rm -rf go go-workspace _go-v$(GO_VERSION)
