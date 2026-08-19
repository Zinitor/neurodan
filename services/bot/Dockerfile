FROM python:3.14-slim

COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/

WORKDIR /app

# Copy dependency files
COPY services/bot/pyproject.toml ./

# Install dependencies
RUN uv sync --frozen --no-install-project

# Copy source code
COPY services/bot/src ./src

# Run the application
CMD ["uv", "run", "python", "src/main.py"]
