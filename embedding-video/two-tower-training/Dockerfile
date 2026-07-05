FROM golang:1.26.1-bookworm

COPY requirements.txt /tmp/two-tower-training-requirements.txt

RUN apt-get update \
    && apt-get install -y --no-install-recommends python3 python3-pip \
    && pip3 install --break-system-packages --no-cache-dir --extra-index-url https://download.pytorch.org/whl/cpu -r /tmp/two-tower-training-requirements.txt \
    && rm -f /tmp/two-tower-training-requirements.txt \
    && rm -rf /var/lib/apt/lists/*
