# Phase A test container shortcuts.
# Override RUNTIME=podman to run rootless, otherwise defaults to docker.

IMG       ?= chezmoi-test:local
RUNTIME   ?= docker
HERE      := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))
REPO_ROOT := $(shell git -C $(HERE) rev-parse --show-toplevel)

.PHONY: build test shell ci-test selftest help

help:
	@echo 'Targets:'
	@echo '  build     Build the test image ($(IMG)) with $(RUNTIME)'
	@echo '  test      Run the smoke entrypoint against $(REPO_ROOT)'
	@echo '  shell     Start an interactive bash inside the test image'
	@echo '  selftest  Run the fake-bw self-test (no container)'
	@echo '  ci-test   Alias for test, used by CI'
	@echo 'Variables:'
	@echo '  RUNTIME   docker (default) or podman'
	@echo '  IMG       image tag (default $(IMG))'

build:
	$(RUNTIME) build -t $(IMG) -f $(HERE)Dockerfile $(HERE)

test: build
	$(RUNTIME) run --rm -v "$(REPO_ROOT):/work:ro" $(IMG)

shell: build
	$(RUNTIME) run --rm -it -v "$(REPO_ROOT):/work:ro" --entrypoint bash $(IMG)

selftest:
	$(HERE)tests/test-fake-bw.sh

ci-test: test
