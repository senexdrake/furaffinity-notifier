# Add linker flags when building for production
# those will strip symbols and debug info
# both "prod" and "production" count as production target
PROD_TARGETS=prod production
ifneq (, $(filter $(TARGET), $(PROD_TARGETS)))
	GO_TAGS := release
	LD_FLAGS := -w -s
else
	GO_TAGS := debug
endif

EXECUTABLE_NAME := fa-notifier
ifeq ($(GOOS), windows)
	EXECUTABLE_NAME := $(EXECUTABLE_NAME).exe
endif

build:
	CGO_ENABLED=1 go build -tags $(GO_TAGS) -o bin/$(EXECUTABLE_NAME) --ldflags="$(LD_FLAGS)"

test:
	go test ./...

deps:
	go mod download && go mod verify

deps-update:
	go get -u && go mod tidy