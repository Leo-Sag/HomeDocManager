#!/bin/bash
echo "Getting access token..."
TOKEN=$(gcloud auth print-access-token)
if [ -z "$TOKEN" ]; then
    echo "Failed to get token"
    exit 1
fi
echo "Logging in to gcr.io..."
echo "$TOKEN" | docker login -u oauth2accesstoken --password-stdin https://gcr.io
