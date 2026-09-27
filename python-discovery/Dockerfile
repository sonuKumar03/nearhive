FROM mcr.microsoft.com/playwright/python:v1.49.1-noble

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1

# Copy project definitions and code
COPY pyproject.toml README.md ./
COPY nearhive_discovery/ ./nearhive_discovery/

# Install package dependencies and binary entrypoint
RUN pip install --upgrade pip && \
    pip install .

# Run as non-root user (pwuser exists in Playwright base image)
USER pwuser

ENTRYPOINT ["python", "-m", "nearhive_discovery.worker"]
