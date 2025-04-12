FROM alpine:3.21.3

LABEL maintainer="timophey@kushnir.msk.ru"

EXPOSE 8080

COPY avatarad.x /usr/local/bin/avatarad

ENTRYPOINT [ "/usr/local/bin/avatarad" ]

HEALTHCHECK --interval=1m --timeout=10s \
	CMD wget -qO- http://localhost:8080/healthz | grep -q '^OK$'
