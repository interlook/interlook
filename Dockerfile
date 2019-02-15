FROM alpine:edge AS build

RUN apk add build-base git go

COPY . /usr/src/interlook

WORKDIR /usr/src/interlook

RUN go build -o build/interlookd cmd/interlookd/main.go

FROM alpine:3.8
LABEL maintainer="Boris HUISGEN <bhuisgen@hbis.fr>"

COPY --from=build /usr/src/interlook/build/ /usr/bin/

ENTRYPOINT ["/usr/bin/interlookd"]
CMD []
