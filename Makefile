ifndef TAG
GIT_COMMIT = $(shell git rev-parse "HEAD^{commit}" 2>/dev/null)
GIT_VERSION = $(shell git describe --tags --match='v*' --abbrev=14 "${GIT_COMMIT}^{commit}"||echo "v0.0.0-unknown")
TAG=$(subst v,,$(GIT_VERSION))
endif

all: image

image:
	@echo "building nano-gpu-scheduler docker image..."
	docker build -t  nano-gpu-scheduler:$(TAG) -f Dockerfile .
