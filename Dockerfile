FROM dockerhub.hanada.info/library/ubuntu:20.04

ENV TZ=Asia/Shanghai
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y curl apt-transport-https xz-utils && \
    curl -L -o "/tmp/packages-microsoft-prod.deb" \
    "https://packages.microsoft.com/config/ubuntu/20.04/packages-microsoft-prod.deb" && \
    dpkg -i /tmp/packages-microsoft-prod.deb && \
    apt-get update && \
    apt-get install -y aspnetcore-runtime-3.1 && \
    ln -fs /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    dpkg-reconfigure -f noninteractive tzdata && \
    curl -L -o "/tmp/linux64.r2r.tar.xz" \
    "https://repo.hanada.info/repositories/danmaku/release/1.0.0-beta11/linux64.r2r.tar.xz" && \
    tar -xvf /tmp/linux64.r2r.tar.xz -C /usr/local && \
    mv /usr/local/Danmu /usr/local/danmaku && \
    apt-get clean && \
    rm -f /tmp/packages-microsoft-prod.deb /tmp/linux64.r2r.tar.xz

WORKDIR /usr/local/danmaku

COPY appsettings.yml appsettings.yml

EXPOSE 80

CMD ["./Danmu"]
