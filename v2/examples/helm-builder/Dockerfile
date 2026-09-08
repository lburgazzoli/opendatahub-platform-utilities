FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY bin/manager /manager
COPY config/chart /opt/charts

USER 65532:65532
ENTRYPOINT ["/manager"]
