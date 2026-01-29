#!/bin/bash

# 1. Start Docker containers in the background
echo "🚀 Starting Docker containers..."
docker compose up -d

# 2. Wait a moment for Postgres to be ready
echo "⏳ Waiting for database to initialize..."
sleep 3 

# 3. Create a fresh backup (timestamped)
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
echo "💾 Creating backup: backups/db_backup_$TIMESTAMP.sql"
mkdir -p backups
docker compose exec -T postgres pg_dump -U toomanyhours toomanyhours > backups/db_backup_$TIMESTAMP.sql

# 4. Run your Go application
echo "🏃 Running Go application..."
go run ./cmd/api