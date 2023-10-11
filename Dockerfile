FROM dockerhub.hanada.info/library/ubuntu:20.04 as builder

ENV TZ=Asia/Shanghai
ENV DEBIAN_FRONTEND=noninteractive

WORKDIR /build

COPY . /build

RUN apt-get update \
    && apt-get install -y \
        curl \
        apt-transport-https \
        xz-utils \
        nodejs \
        npm \
    && ln -fs /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && dpkg-reconfigure -f noninteractive tzdata \
    && curl -L -o /tmp/packages-microsoft-prod.deb \
        https://packages.microsoft.com/config/ubuntu/20.04/packages-microsoft-prod.deb \
    && dpkg -i /tmp/packages-microsoft-prod.deb \
    && add-apt-repository universe \
    && apt-get update \
    && apt-get install -y dotnet-sdk-3.1 \
    && cd /build/Danmu \
    && dotnet remove package Microsoft.VisualStudio.Web.CodeGeneration.Design \
    && dotnet remove package VueCliMiddleware \
    && cd clientapp \
    && npm install \
    && npm run build \
    && cd /build/danmaku \
    && CLI_VERSION=`git describe --tags` \
    && sed -i "s/<PublishReadyToRun>false/<PublishReadyToRun>true/g" /build/Danmu/Danmu.csproj \
    && sed -i "s/<PublishReadyToRun>false/<PublishReadyToRun>true/g" /build/Danmu/Danmu.csproj \
    && mkdir /output \
    && if [ "$TARGETARCH" == "amd64" ]; then \
        dotnet publish \
        "/build/Danmu/Danmu.csproj" \
        -c Release-Linux64 \
        -r linux-x64 \
        --self-contained false \
        --output /output; \
       elif [ "$TARGETARCH" == "arm64" ]; then \
        dotnet publish \
        "/build/Danmu/Danmu.csproj" \
        -c Release-Linux64 \
        -r linux-arm64 \
        --self-contained false \
        --output /output; \
       fi

FROM dockerhub.hanada.info/library/ubuntu:20.04 as runner

ENV TZ=Asia/Shanghai
ENV DEBIAN_FRONTEND=noninteractive

WORKDIR /usr/local/danmaku

COPY --from=builder /output /usr/local/danmaku

RUN apt-get update \
    && apt-get install -y curl apt-transport-https xz-utils \
    && curl -L -o "/tmp/packages-microsoft-prod.deb" \
        https://packages.microsoft.com/config/ubuntu/20.04/packages-microsoft-prod.deb \
    && dpkg -i /tmp/packages-microsoft-prod.deb \
    && apt-get update \
    && apt-get install -y aspnetcore-runtime-3.1 \
    && ln -fs /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && dpkg-reconfigure -f noninteractive tzdata \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* \
    && rm -f /tmp/packages-microsoft-prod.deb

WORKDIR /usr/local/danmaku

EXPOSE 80

CMD ["./Danmu"]
