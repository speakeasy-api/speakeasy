FROM debian:12-slim

# Install only sudo (required by Speakeasy CLI)
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    sudo \
    && rm -rf /var/lib/apt/lists/*

# Create a non-root user with sudo access
RUN groupadd -g 1001 speakeasy && \
    useradd -u 1001 -g speakeasy -m -s /bin/bash speakeasy && \
    echo 'speakeasy ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

# Copy the binary from GoReleaser build context
# GoReleaser will automatically use the correct binary for the target architecture
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/speakeasy /usr/local/bin/speakeasy

# Make the binary executable
RUN chmod +x /usr/local/bin/speakeasy

USER speakeasy

# Default entrypoint
ENTRYPOINT ["speakeasy"]
CMD ["--version"]