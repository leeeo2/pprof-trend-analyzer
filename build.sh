#!/bin/bash

set -e

echo "🔧 Building Pprof Trend Analyzer..."

# Build frontend
echo "📦 Building frontend..."
cd frontend
npm install
npm run build
cd ..

# Build backend
echo "🔨 Building backend..."
go build -o pprof-analyzer main.go

echo "✅ Build completed!"
echo ""
echo "To run the application:"
echo "  ./pprof-analyzer"
echo ""
echo "Then open http://localhost:8080 in your browser"
