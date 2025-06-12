#!/bin/bash
# Set the base URL
HERMYX_URL="http://localhost:8080"
RATE=44000           # requests per second
DURATION=10s       # test duration
TIMESTAMP=$(date +"%Y%m%d-%H%M%S")
TARGETS_FILE="hermyx.targets"
RESULTS_BIN="results-$TIMESTAMP.bin"
REPORT_FILE="report-$TIMESTAMP.txt"

# Create the targets file
cat > $TARGETS_FILE <<EOF
GET $HERMYX_URL/hello
GET $HERMYX_URL/time
GET $HERMYX_URL/exceed
GET $HERMYX_URL/echo?msg=first
GET $HERMYX_URL/echo?msg=second
GET $HERMYX_URL/echo?msg=first
EOF

echo "📌 Running Vegeta benchmark at $RATE RPS for $DURATION..."
vegeta attack -rate=$RATE  -keepalive=true -duration=$DURATION -targets=$TARGETS_FILE | tee $RESULTS_BIN | vegeta report | tee $REPORT_FILE

echo "✅ Benchmark complete."
echo "📝 Report saved to: $REPORT_FILE"
echo "📦 Raw data saved to: $RESULTS_BIN"

