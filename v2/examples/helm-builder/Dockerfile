FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY bin/manager /manager
COPY config/manager/chart /charts/component

USER 65532:65532
ENTRYPOINT ["/manager"]
