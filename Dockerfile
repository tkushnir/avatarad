FROM alpine:3.15

LABEL maintainer="timophey@kushnir.msk.ru"

EXPOSE 8080

COPY avatarad.x /usr/local/bin/avatarad

ENTRYPOINT [ "/usr/local/bin/avatarad" ]
