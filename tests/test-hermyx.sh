#!/bin/bash

HERMYX_URL="http://localhost:8080"

echo "Testing /hello endpoint (should cache)"
curl -i "$HERMYX_URL/hello"
echo -e "\n"
sleep 1
curl -i "$HERMYX_URL/hello"
echo -e "\n\n"

echo "Testing /time endpoint (cache TTL 10s)"
curl -i "$HERMYX_URL/time"
echo -e "\n"
sleep 2
curl -i "$HERMYX_URL/time"
echo -e "\n\n"

echo "Testing /delay endpoint (5 second delay)"
time curl -i "$HERMYX_URL/delay"
echo -e "\n\n"


echo "Testing cached /delay endpoint (5 second delay)"
time curl -i "$HERMYX_URL/delay"
echo -e "\n\n"


echo "Testing exceeded content size /exceed endpoint"
time curl -i "$HERMYX_URL/exceed"
echo -e "\n\n"

echo "Testing /echo endpoint with query param (cache key includes query)"
curl -i "$HERMYX_URL/echo?msg=first"
echo -e "\n"
curl -i "$HERMYX_URL/echo?msg=second"
echo -e "\n"
curl -i "$HERMYX_URL/echo?msg=first"  # should be cached
echo -e "\n"

