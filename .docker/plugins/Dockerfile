FROM alpine:3.21

RUN apk update
RUN apk add xfsprogs e2fsprogs ca-certificates nvme-cli

RUN mkdir -p /lib64 && ln -s /lib/libc.musl-aarch64.so.1 /lib64/ld-linux-x86-64.so.2 && ln -s /lib/libc.musl-aarch64.so.1 /lib64/ld-linux-aarch64.so.2
RUN ln -s /lib/libc.musl-aarch64.so.1 /lib/ld-linux-aarch64.so.1
RUN mkdir -p /etc/rexray /run/docker/plugins /var/lib/rexray/volumes
ADD rexray /usr/bin/rexray
ADD rexray.yml /etc/rexray/rexray.yml

ADD rexray.sh /rexray.sh
RUN chmod +x /rexray.sh /usr/bin/rexray

CMD [ "rexray", "start", "--nopid" ]
ENTRYPOINT [ "/rexray.sh" ]
