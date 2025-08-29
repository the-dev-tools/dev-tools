#!/bin/bash

# Test HeaderDeltaList RPC call

echo "=== Current Database State ==="
sqlite3 development.db "SELECT hex(id) as id, name, CASE WHEN version_parent_id IS NULL THEN 'ORIGIN' ELSE 'DELTA' END as type FROM item_api_example;"

echo ""
echo "=== Testing HeaderDeltaList ==="
echo ""

# Get the IDs
ORIGIN_ID="0198F0811938831D3B31F15CC3699301"
# The delta should be the one WITH a version_parent_id
DELTA_ID="0198F1B4ED428B796073510A20575947"

echo "Using:"
echo "  Origin ID: $ORIGIN_ID (should have headers)"
echo "  Delta ID:  $DELTA_ID (should have version_parent_id)"

# Note: You would need to call this through your actual RPC endpoint
# This is just showing the correct IDs to use
echo ""
echo "Call HeaderDeltaList with:"
echo "  exampleId: $DELTA_ID"
echo "  originId: $ORIGIN_ID"