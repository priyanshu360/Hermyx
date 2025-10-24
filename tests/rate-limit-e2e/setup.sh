#!/bin/bash

# E2E Test Setup Script
# This script sets up the environment for rate limiting e2e tests

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    if [ "$status" = "PASS" ]; then
        echo -e "${GREEN}✓ PASS${NC}: $message"
    elif [ "$status" = "FAIL" ]; then
        echo -e "${RED}✗ FAIL${NC}: $message"
    elif [ "$status" = "INFO" ]; then
        echo -e "${BLUE}ℹ INFO${NC}: $message"
    elif [ "$status" = "WARN" ]; then
        echo -e "${YELLOW}⚠ WARN${NC}: $message"
    fi
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to check Docker and Docker Compose
check_docker() {
    print_status "INFO" "Checking Docker installation..."
    
    if ! command_exists docker; then
        print_status "FAIL" "Docker is not installed. Please install Docker first."
        exit 1
    fi
    
    if ! command_exists docker-compose; then
        print_status "FAIL" "Docker Compose is not installed. Please install Docker Compose first."
        exit 1
    fi
    
    # Check if Docker daemon is running
    if ! docker info >/dev/null 2>&1; then
        print_status "FAIL" "Docker daemon is not running. Please start Docker first."
        exit 1
    fi
    
    print_status "PASS" "Docker and Docker Compose are available"
}

# Function to check curl
check_curl() {
    print_status "INFO" "Checking curl installation..."
    
    if ! command_exists curl; then
        print_status "FAIL" "curl is not installed. Please install curl first."
        exit 1
    fi
    
    print_status "PASS" "curl is available"
}

# Function to create necessary directories
create_directories() {
    print_status "INFO" "Creating necessary directories..."
    
    # Create logs directory
    mkdir -p logs
    
    # Create test results directory
    mkdir -p test-results
    
    print_status "PASS" "Directories created successfully"
}

# Function to clean up existing containers
cleanup_existing() {
    print_status "INFO" "Cleaning up existing containers..."
    
    # Stop and remove existing containers
    docker-compose down --remove-orphans >/dev/null 2>&1 || true
    
    # Remove any dangling containers
    docker container prune -f >/dev/null 2>&1 || true
    
    print_status "PASS" "Existing containers cleaned up"
}

# Function to start Docker services
start_docker_services() {
    print_status "INFO" "Starting Docker services (Redis and Mock Server)..."
    
    # Start only Redis and Mock Server services
    if docker-compose up -d redis mock-server; then
        print_status "PASS" "Docker services started successfully"
    else
        print_status "FAIL" "Failed to start Docker services"
        exit 1
    fi
}

# Function to start services
start_services() {
    print_status "INFO" "Starting services..."
    
    # Start Docker services
    start_docker_services
}

# Function to wait for services to be ready
wait_for_services() {
    print_status "INFO" "Waiting for Docker services to be ready..."
    
    local max_attempts=30
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        local all_ready=true
        
        # Check Redis
        if ! docker exec hermyx-redis redis-cli ping >/dev/null 2>&1; then
            all_ready=false
        fi
        
        # Check Mock Server
        if ! curl -s http://localhost:8081/status/200 >/dev/null 2>&1; then
            all_ready=false
        fi
        
        if [ "$all_ready" = true ]; then
            print_status "PASS" "Docker services are ready"
            return 0
        fi
        
        echo -n "."
        sleep 2
        ((attempt++))
    done
    
    print_status "FAIL" "Docker services are not ready after $max_attempts attempts"
    return 1
}

# Function to verify services
verify_services() {
    print_status "INFO" "Verifying Docker services..."
    
    # Test Redis
    if docker exec hermyx-redis redis-cli ping | grep -q "PONG"; then
        print_status "PASS" "Redis is responding"
    else
        print_status "FAIL" "Redis is not responding"
        return 1
    fi
    
    # Test Mock Server
    if curl -s -w "%{http_code}" http://localhost:8081/status/200 | grep -q "200"; then
        print_status "PASS" "Mock Server is responding"
    else
        print_status "FAIL" "Mock Server is not responding"
        return 1
    fi
}

# Function to show service status
show_status() {
    print_status "INFO" "Docker Service Status:"
    echo ""
    echo "Redis: localhost:6379"
    echo "Mock Server: http://localhost:8081"
    echo ""
    echo "Docker Compose Services:"
    docker-compose ps
    echo ""
    echo "Note: Hermyx will be built and run by the test scripts"
}

# Main setup function
main() {
    echo "=========================================="
    echo "E2E Test Environment Setup"
    echo "=========================================="
    
    # Check prerequisites
    check_docker
    check_curl
    
    # Create directories
    create_directories
    
    # Clean up existing containers
    cleanup_existing
    
    # Start Docker services
    start_services
    
    # Wait for services to be ready
    wait_for_services
    
    # Verify services
    verify_services
    
    # Show status
    show_status
    
    print_status "PASS" "Docker services setup completed successfully!"
    echo ""
    echo "Docker services are ready:"
    echo "  - Redis: localhost:6379"
    echo "  - Mock Server: http://localhost:8081"
    echo ""
    echo "You can now run the tests:"
    echo "  ./memory-rate-limit-e2e-tests.sh"
    echo "  ./redis-rate-limiter-e2e-tests.sh"
    echo ""
    echo "Note: Test scripts will build and run Hermyx directly"
    echo ""
    echo "To stop the environment:"
    echo "  ./teardown.sh"
}

# Run main function
main "$@"
