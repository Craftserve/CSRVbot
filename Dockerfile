FROM golang:1.17-alpine3.15 as builder

ADD . /src

RUN cd /src && go build -o csrvbot

FROM alpine:3.15

WORKDIR /app
RUN chown nobody:nogroup /app

COPY --from=builder --chown=nobody:nogroup /src ./
RUN chmod +x csrvbot
USER nobody

CMD ["./csrvbot"]