## Build
FROM golang:1.22-alpine3.20 as builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY *.go ./
#COPY internal/ ./internal/

RUN go build -o /runner

## Deploy
FROM golang:1.22-alpine3.20

RUN apk update
RUN apk add git

### Install Node / NPM
RUN apk add --update --no-cache nodejs npm
RUN npm install -g pnpm # install pnpm

### Install Python
RUN apk add --update --no-cache python3 py3-pip python3-dev

### Install Poetry and validate
RUN apk add --update --no-cache poetry
RUN poetry --version

### Install Java
RUN apk add --update --no-cache openjdk11 gradle

### Install Ruby
RUN apk add --update --no-cache build-base ruby ruby-bundler ruby-dev

### Install .NET6.0
ENV DOTNET_ROOT=/usr/lib/dotnet
RUN apk add --update --no-cache dotnet6-sdk

### Install .NET5.0
RUN apk add --update --no-cache curl bash
# openssl1.1-compat is gradually getting removed from package managers..
RUN apk add --update --no-cache --repository=http://dl-cdn.alpinelinux.org/alpine/edge/testing openssl1.1-compat
RUN curl -sSL https://dot.net/v1/dotnet-install.sh | bash /dev/stdin -Channel 5.0 -InstallDir ${DOTNET_ROOT}

### Install .NET8.0
RUN curl -sSL https://dot.net/v1/dotnet-install.sh | bash /dev/stdin -Channel 8.0 -InstallDir ${DOTNET_ROOT}
RUN dotnet --list-sdks

### Install PHP and Composer
#### Source: https://github.com/geshan/docker-php-composer-alpine/blob/master/Dockerfile
RUN apk --update --no-cache add wget \
		     curl \
		     git \
		     php82 \
         php-ctype php-dom php-json php-mbstring php-phar php-tokenizer php-xml php-xmlwriter \
		     php-curl \
		     php-openssl \
		     php-iconv \
		    --repository http://nl.alpinelinux.org/alpine/edge/testing/

RUN curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/bin --filename=composer
RUN mkdir -p /var/www
WORKDIR /var/www
COPY . /var/www
VOLUME /var/www
### END PHP

### Install Speakeasy CLI
RUN apk add sudo
RUN curl -fsSL https://raw.githubusercontent.com/speakeasy-api/speakeasy/main/install.sh | sh

WORKDIR /

COPY --from=builder /runner /runner

ENTRYPOINT ["/runner"]