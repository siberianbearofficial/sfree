ARG PYTHON_IMAGE=python:3.12-slim
FROM ${PYTHON_IMAGE}

ARG MC_RELEASE=RELEASE.2025-01-17T23-25-50Z
ARG TARGETARCH

WORKDIR /work
COPY e2e/requirements.txt /tmp/requirements.txt
RUN pip install --no-cache-dir -r /tmp/requirements.txt
RUN case "$TARGETARCH" in \
        amd64|arm64) MC_ARCH="$TARGETARCH" ;; \
        *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; \
    esac \
    && MC_URL="https://dl.min.io/client/mc/release/linux-${MC_ARCH}/archive/mc.${MC_RELEASE}" \
    && python3 -c "import sys, urllib.request; urllib.request.urlretrieve(sys.argv[1], '/usr/local/bin/mc')" "$MC_URL" \
    && chmod +x /usr/local/bin/mc \
    && /usr/local/bin/mc --version
COPY e2e /work/e2e
COPY e2e/pytest.ini /work
ENV PYTHONPATH=/work
CMD ["pytest", "-q", "e2e/test_api_e2e.py"]
