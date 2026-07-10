FROM golang:1.26.1-bookworm

WORKDIR /repo/recbole-training

RUN apt-get update \
    && apt-get install -y --no-install-recommends python3 python3-venv python3-pip \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN python3 -m venv /opt/recbole-venv \
    && /opt/recbole-venv/bin/pip install --no-cache-dir --upgrade pip \
    && /opt/recbole-venv/bin/pip install --no-cache-dir -r requirements.txt

ENV PATH="/opt/recbole-venv/bin:${PATH}"

COPY . .
