ARG WORKDIR=/opt/app

FROM golang:1.22-alpine as builder
ARG WORKDIR
# Set Target to production for Makefile
ENV TARGET prod
WORKDIR $WORKDIR

# make is needed for the Makefile
RUN apk add --no-cache --update make build-base

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum Makefile ./
RUN make deps

COPY . .
# Run go build and strip symbols / debug info
RUN make build

FROM alpine
ARG WORKDIR
ENV APP_ENV production
WORKDIR $WORKDIR
COPY --from=builder $WORKDIR/bin/fa-notifier .

EXPOSE 3000

ENTRYPOINT ["./fa-notifier"]