#!/bin/bash

echo "Setting up WhatsApp API Wrapper..."

# Install Go dependencies
echo "Installing Go dependencies..."
cd whatsmeow
go mod tidy
cd ..
go mod tidy

# Create Python virtual environment
echo "Creating Python virtual environment..."
python3 -m venv venv
source venv/bin/activate

# Install Python dependencies
echo "Installing Python dependencies..."
pip install -r requirements.txt

echo "Setup complete!"
echo ""
echo "To run the services:"
echo "1. Start Go service: ./run-go.sh"
echo "2. Start Python API: ./run-python.sh"
echo ""
echo "The Python API will be available at http://localhost:8000"
echo "API documentation at http://localhost:8000/docs"