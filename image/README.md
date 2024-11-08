# CLI Docker Image

Builds a Docker image for the CLI. Note that, though it is colocated with the CLI source code, it does not actually
build the CLI directly.
Instead, the image downloads and install the CLI from the latest release. This is done so that it most closely mirrors
GitHub Actions, local
installations, etc.

## Cloud Build

At the time of writing, this image is built by Cloud Build on every new tag release of the CLI repository.

## Cloud Run

At the time of writing, the image built by Cloud Build is used by the "Remote CLI" cloud run job. The `latest` tag is
pulled each time a new job is spun up.