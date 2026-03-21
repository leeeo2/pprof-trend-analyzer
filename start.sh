#!/bin/bash

echo "🚀 Starting Pprof Trend Analyzer..."

# Check if frontend is built
if [ ! -d "frontend/dist" ]; then
    echo "📦 Frontend not built. Building now..."
    cd frontend
    npm install
    npm run build
    cd ..
fi

# Start the server
echo "🔥 Starting server on http://localhost:8080"
go run main.go
