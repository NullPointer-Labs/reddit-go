FROM python:3.13-slim
WORKDIR /app
RUN pip install --no-cache-dir flask==3.1.0 sqlalchemy==2.0.36 "psycopg[binary]==3.2.3" redis==5.2.1 gunicorn==23.0.0
COPY app.py .
CMD ["gunicorn", "-w", "4", "-b", "0.0.0.0:8000", "--access-logfile", "-", "app:app"]