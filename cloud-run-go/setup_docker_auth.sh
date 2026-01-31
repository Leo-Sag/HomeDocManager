#!/bin/bash
TOKEN=$(cat access_token.txt | tr -d '[:space:]')
if [ -z "$TOKEN" ]; then
    echo "Token not found"
    exit 1
fi
AUTH_STRING="oauth2accesstoken:$TOKEN"
AUTH_BASE64=$(echo -n "$AUTH_STRING" | base64 | tr -d '\n')
cat > docker_auth_config.json <<EOF
{
  "auths": {
    "https://gcr.io": {
      "auth": "$AUTH_BASE64"
    }
  },
  "credsStore": ""
}
EOF
mkdir -p ~/.docker
cp docker_auth_config.json ~/.docker/config.json
echo "Docker config updated manually."
