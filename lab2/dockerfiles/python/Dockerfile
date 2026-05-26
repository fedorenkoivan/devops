FROM python:3.13-bookworm

WORKDIR /app

COPY . .

RUN pip install --no-cache-dir fastapi "pydantic>=2.0" pydantic-settings starlette "uvicorn[standard]"

EXPOSE 8000

CMD ["uvicorn", "spaceship.main:app", "--host", "0.0.0.0", "--port", "8000"]
