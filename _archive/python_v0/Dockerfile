# ベースイメージ
# ベースイメージ
FROM python:3.11-slim

# poppler-utilsのインストール（PDF処理に必須）
RUN apt-get update && apt-get install -y \
    poppler-utils \
    && rm -rf /var/lib/apt/lists/*

# 作業ディレクトリ
WORKDIR /app

# 依存関係のインストール
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# アプリケーションコードのコピー
COPY . .

# Cloud Runのポート設定
ENV PORT=8080

# Gunicornで起動
CMD exec gunicorn --bind :$PORT --workers 1 --threads 8 --timeout 0 main:app
