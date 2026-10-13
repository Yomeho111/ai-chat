FROM python:3.10-slim
WORKDIR /app
ENV PORT=3002 \
    TIKTOKEN_CACHE_DIR=/app/tiktoken-cache \
    PYTHONUNBUFFERED=1

COPY ./requirements.txt /app/
RUN pip install --no-cache-dir -r requirements.txt \
    && python -c "import tiktoken; tiktoken.get_encoding('cl100k_base')"

COPY ./tokenizer.py /app/

CMD ["sh", "-c", "exec nuxt --address 0.0.0.0 --port ${PORT} --module tokenizer.py --workers 2"]