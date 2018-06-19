FROM golang:1.9 as builder
COPY main.go src/
RUN GO_EXTLINK_ENABLED=0 CGO_ENABLED=0 go build \
    -ldflags "-w -extldflags -static" \
    -tags netgo -installsuffix netgo \
    -o /spec ./src/main.go

FROM scratch
COPY --from=builder /spec /spec
ENTRYPOINT [ "/spec" ]