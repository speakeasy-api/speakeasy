FROM alpine:3.21

RUN apk update

### Install common tools
RUN apk add --update --no-cache bash curl git wget

### Install Node / NPM
RUN apk add --update --no-cache nodejs npm

### Install Python
RUN apk add --update --no-cache python3 py3-pip python3-dev pipx

### Install Java
RUN apk add --update --no-cache openjdk11 gradle

### Install Ruby
#### gcompat is required on Alpine linux to support gcc ruby packages like sorbet
RUN apk add --update --no-cache build-base ruby ruby-bundler ruby-dev gcompat

### Install .NET
ENV DOTNET_ROOT=/usr/lib/dotnet
RUN apk add --update --no-cache dotnet8-sdk
RUN curl -sSL https://dot.net/v1/dotnet-install.sh | bash /dev/stdin -Channel 6.0 -InstallDir ${DOTNET_ROOT}
RUN dotnet --list-sdks

### Install PHP and Composer
#### Source: https://github.com/geshan/docker-php-composer-alpine/blob/master/Dockerfile
RUN apk --update --no-cache add \
	php83 \
	php83-ctype \
	php83-dom \
	php83-json \
	php83-mbstring \
	php83-phar \
	php83-tokenizer \
	php83-xml \
	php83-xmlwriter \
	php83-curl \
	php83-openssl \
	php83-iconv \
	php83-session \
	php83-fileinfo \
	--repository http://nl.alpinelinux.org/alpine/edge/testing/


RUN curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/bin --filename=composer
RUN mkdir -p /var/www
WORKDIR /var/www
COPY . /var/www
VOLUME /var/www
### END PHP

# Install sudo and CA certificates (required for TLS/SSL connections)
RUN apk add --no-cache sudo ca-certificates

# Create a non-root user with sudo access
RUN addgroup -g 1001 speakeasy && \
    adduser -u 1001 -G speakeasy -D -s /bin/sh speakeasy && \
    echo 'speakeasy ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

# Copy the binary from GoReleaser build context
# GoReleaser will automatically use the correct binary for the target architecture
COPY speakeasy /usr/local/bin/speakeasy

# Make the binary executable (must be done as root before switching users)
RUN chmod +x /usr/local/bin/speakeasy

USER speakeasy

# Default entrypoint
ENTRYPOINT ["speakeasy"]
CMD ["--version"]