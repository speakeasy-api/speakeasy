FROM alpine:3.21

# Install all system packages in a single layer for better caching and smaller image
RUN apk update && apk add --no-cache \
    # Common tools
    bash \
    curl \
    git \
    wget \
    # Node.js and NPM
    nodejs \
    npm \
    # Python
    python3 \
    py3-pip \
    python3-dev \
    pipx \
    # Java
    openjdk11 \
    gradle \
    # Ruby (gcompat required for gcc ruby packages like sorbet)
    build-base \
    ruby \
    ruby-bundler \
    ruby-dev \
    gcompat \
    # .NET
    dotnet8-sdk \
    # PHP and extensions
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
    # System utilities
    sudo \
    ca-certificates \
    --repository http://nl.alpinelinux.org/alpine/edge/testing/ && \
    rm -rf /var/cache/apk/*

# Install .NET 6.0 SDK (in addition to 8.0) and verify installation
ENV DOTNET_ROOT=/usr/lib/dotnet
RUN curl -sSL https://dot.net/v1/dotnet-install.sh | bash /dev/stdin -Channel 6.0 -InstallDir ${DOTNET_ROOT} && \
    dotnet --list-sdks

# Install Composer
RUN curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/bin --filename=composer

# Install Python package managers: uv and poetry via pipx
RUN pipx install uv && pipx install poetry

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