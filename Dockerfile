# Building 

FROM golang:1.21-alpine3.19 as builder
WORKDIR /blazectl
COPY ./cmd /blazectl/cmd
COPY ./data /blazectl/data
COPY ./fhir /blazectl/fhir
COPY ./util /blazectl/util
COPY ./go.mod /blazectl/go.mod
COPY ./go.sum /blazectl/go.sum
COPY ./main.go /blazectl/main.go
COPY ./LICENSE /blazectl/LICENSE

# certs business
RUN apk --no-cache add ca-certificates

RUN go build -o /go/bin/blazectl
USER nonroot

# Deployment 
FROM alpine:3.19
RUN addgroup -S nonroot \
    && adduser -S nonroot -G nonroot
WORKDIR /app
COPY --from=builder /go/bin/blazectl /app/blazectl
ENTRYPOINT [ "/app/blazectl" ]
CMD [ "help" ]
USER nonroot
