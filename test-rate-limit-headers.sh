#!/bin/bash

echo "🧪 Testing Rate Limit Headers"
echo "================================"
echo ""

# Test 1: First request
echo "📍 Test 1: First request (should see headers)"
echo "---"
curl -s -i http://localhost:8080/api/public | grep -E "(X-RateLimit-|HTTP/)"
echo ""
echo ""

# Test 2: Second request
echo "📍 Test 2: Second request (should see decremented remaining)"
echo "---"
curl -s -i http://localhost:8080/api/public | grep -E "(X-RateLimit-|HTTP/)"
echo ""
echo ""

# Test 3: Check all headers
echo "📍 Test 3: All headers from response"
echo "---"
curl -s -I http://localhost:8080/api/public
echo ""


