FROM python:3.14-slim

COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/

WORKDIR /app

# Copy dependency files
COPY services/generator/pyproject.toml ./

# Install dependencies
RUN uv sync --frozen --no-install-project

# Copy source code
COPY services/generator/src ./src

# Run the application
CMD ["uv", "run", "uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8000"]
