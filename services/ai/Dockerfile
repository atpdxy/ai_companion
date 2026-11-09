FROM python:3.12-slim
WORKDIR /app
COPY pyproject.toml ./
COPY requirements.lock ./
RUN pip install --no-cache-dir -r requirements.lock
COPY src ./src
RUN pip install --no-cache-dir --no-deps . && useradd --create-home companion
USER companion
EXPOSE 8000
CMD ["uvicorn", "emotional_agent.api.main:app", "--host", "0.0.0.0", "--port", "8000", "--no-access-log"]
