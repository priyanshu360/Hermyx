#!/bin/bash

# E2E Test Teardown Script
# This script cleans up the e2e test environment

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

# Function to stop and remove containers
stop_containers() {
    print_status "INFO" "Stopping and removing Docker containers..."
    
    # Stop Docker services (Redis and Mock Server)
    if docker-compose down --remove-orphans; then
        print_status "PASS" "Docker containers stopped successfully"
    else
        print_status "WARN" "Some containers may not have stopped cleanly"
    fi
}

# Function to clean up Docker resources
cleanup_docker() {
    print_status "INFO" "Cleaning up Docker resources..."
    
    # Remove dangling containers
    if docker container prune -f >/dev/null 2>&1; then
        print_status "PASS" "Dangling containers removed"
    else
        print_status "WARN" "No dangling containers to remove"
    fi
    
    # Remove unused networks
    if docker network prune -f >/dev/null 2>&1; then
        print_status "PASS" "Unused networks removed"
    else
        print_status "WARN" "No unused networks to remove"
    fi
    
    # Remove unused volumes (be careful with this)
    print_status "INFO" "Checking for unused volumes..."
    local unused_volumes=$(docker volume ls -q -f dangling=true | wc -l)
    if [ $unused_volumes -gt 0 ]; then
        print_status "WARN" "Found $unused_volumes unused volumes. Run 'docker volume prune -f' to remove them."
    else
        print_status "PASS" "No unused volumes found"
    fi
}

# Function to clean up test artifacts
cleanup_artifacts() {
    print_status "INFO" "Cleaning up test artifacts..."
    
    # Remove test result files
    if [ -f "memory-test-results.txt" ]; then
        rm -f memory-test-results.txt
        print_status "PASS" "Memory test results removed"
    fi
    
    if [ -f "redis-test-results.txt" ]; then
        rm -f redis-test-results.txt
        print_status "PASS" "Redis test results removed"
    fi
    
    # Clean up logs (optional - keep for debugging)
    if [ -d "logs" ]; then
        print_status "INFO" "Logs directory exists. Remove manually if needed."
    fi
}

# Function to show cleanup summary
show_summary() {
    print_status "INFO" "Cleanup Summary:"
    echo ""
    echo "✓ Docker containers stopped and removed"
    echo "✓ Docker resources cleaned up"
    echo "✓ Test artifacts cleaned up"
    echo ""
    echo "Note: Hermyx processes are managed by test scripts"
    echo ""
    echo "To completely remove all Docker resources:"
    echo "  docker system prune -a --volumes"
    echo ""
    echo "To rebuild the environment:"
    echo "  ./setup.sh"
}

# Function to force cleanup (more aggressive)
force_cleanup() {
    print_status "INFO" "Performing force cleanup..."
    
    # Kill all containers
    docker kill $(docker ps -q) >/dev/null 2>&1 || true
    
    # Remove all containers
    docker rm $(docker ps -aq) >/dev/null 2>&1 || true
    
    # Remove all images
    docker rmi $(docker images -q) >/dev/null 2>&1 || true
    
    # Remove all volumes
    docker volume rm $(docker volume ls -q) >/dev/null 2>&1 || true
    
    # Remove all networks
    docker network rm $(docker network ls -q) >/dev/null 2>&1 || true
    
    print_status "PASS" "Force cleanup completed"
}

# Main teardown function
main() {
    echo "=========================================="
    echo "E2E Test Environment Teardown"
    echo "=========================================="
    
    # Check if we're in the right directory
    if [ ! -f "docker-compose.yml" ]; then
        print_status "FAIL" "docker-compose.yml not found. Are you in the right directory?"
        exit 1
    fi
    
    # Stop containers
    stop_containers
    
    # Clean up Docker resources
    cleanup_docker
    
    # Clean up test artifacts
    cleanup_artifacts
    
    # Show summary
    show_summary
    
    print_status "PASS" "E2E test environment teardown completed successfully!"
}

# Handle command line arguments
case "${1:-}" in
    --force)
        force_cleanup
        ;;
    --help)
        echo "Usage: $0 [--force] [--help]"
        echo ""
        echo "Options:"
        echo "  --force    Perform aggressive cleanup (removes all Docker resources)"
        echo "  --help     Show this help message"
        echo ""
        echo "Default behavior:"
        echo "  - Stop and remove containers"
        echo "  - Clean up unused Docker resources"
        echo "  - Remove test artifacts"
        ;;
    *)
        main "$@"
        ;;
esac
