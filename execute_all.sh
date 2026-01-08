#!/bin/bash

# Color definitions
COLOR_RESET='\033[0m'
COLOR_BLUE='\033[1;34m'
COLOR_GREEN='\033[1;32m'
COLOR_YELLOW='\033[1;33m'
COLOR_CYAN='\033[0;36m'

# Function to print colored command
print_command() {
  echo -e "${COLOR_BLUE}$1${COLOR_RESET}"
}

# Function to print section header
print_header() {
  echo -e "${COLOR_YELLOW}$1${COLOR_RESET}"
}

# Check if project_id and name parameters are provided
if [ -z "$1" ] || [ -z "$2" ]; then
  echo "Error: project_id and name parameters are required"
  echo "Usage: $0 <project_id> <name>"
  exit 1
fi

export PROJECT_ID="$1"
export NAME="$2"

# File paths (can be customized)
export PRIVATE_KEY_FILE="${PRIVATE_KEY_FILE:-private_key_$NAME.pem}"
export PUBLIC_KEY_FILE="${PUBLIC_KEY_FILE:-public_key_$NAME.pem}"
export JWK_FILE="${JWK_FILE:-public_key_$NAME.jwk}"
export JWKS_FILE="${JWKS_FILE:-public_key_$NAME.jwks}"
export EXTERNAL_TOKEN_FILE="${EXTERNAL_TOKEN_FILE:-external_token_$NAME.jwt}"
export GCP_ACCESS_TOKEN_FILE="${GCP_ACCESS_TOKEN_FILE:-gcp_access_token_$NAME.txt}"
export KEY_ID="${KEY_ID:-key-1}"

# Ensure NAME is at least 6 characters
if [ ${#NAME} -lt 6 ]; then
  echo "Error: name parameter must be at least 6 characters"
  echo "Usage: $0 <project_id> <name>"
  exit 1
fi

print_command "gcloud projects create $PROJECT_ID"
gcloud projects create $PROJECT_ID

print_command "gcloud config set project $PROJECT_ID"
gcloud config set project $PROJECT_ID

export PROJECT_NUMBER=$(gcloud projects describe $PROJECT_ID --format="value(projectNumber)")
echo "Project ID: $PROJECT_ID"
echo "Project Number: $PROJECT_NUMBER"

# Enable APIs
print_command "gcloud services enable iamcredentials.googleapis.com sts.googleapis.com pubsub.googleapis.com --project $PROJECT_ID"
gcloud services enable iamcredentials.googleapis.com sts.googleapis.com pubsub.googleapis.com --project $PROJECT_ID

# Create pool
echo "Creating Workload Identity Pool..."
print_command "gcloud iam workload-identity-pools create $NAME --location=global --project $PROJECT_ID"
eval gcloud iam workload-identity-pools create $NAME --location=global --project $PROJECT_ID

echo "Workload Identity Pool created."

echo ""
print_header "========================================="
print_header "Generating keys..."
print_header "========================================="
print_command "go run cmd/generate-keys/main.go --private-key $PRIVATE_KEY_FILE --public-key $PUBLIC_KEY_FILE"
echo "-----------------------------------------"
go run cmd/generate-keys/main.go --private-key "$PRIVATE_KEY_FILE" --public-key "$PUBLIC_KEY_FILE"
echo ""

print_header "========================================="
print_header "Generating JWK..."
print_header "========================================="
print_command "go run cmd/generate-jwk/main.go --key-id $KEY_ID --public-key $PUBLIC_KEY_FILE --jwk-output $JWK_FILE --jwks-output $JWKS_FILE"
echo "-----------------------------------------"
go run cmd/generate-jwk/main.go --key-id "$KEY_ID" --public-key "$PUBLIC_KEY_FILE" --jwk-output "$JWK_FILE" --jwks-output "$JWKS_FILE"
echo ""

echo "Creating Workload Identity Provider..."
# Create provider with inline JWK
export JWKS_CONTENT=$(cat "$JWKS_FILE")
print_command "gcloud iam workload-identity-pools providers create-oidc external-jwt-provider-$NAME --location=global --workload-identity-pool=$NAME --issuer-uri=\"https://my-external-idp.example.com\" --allowed-audiences=\"gcp-workload-identity\" --attribute-mapping=\"google.subject=assertion.sub\" --jwk-json-path=<(echo \"\$JWKS_CONTENT\") --project $PROJECT_ID"
gcloud iam workload-identity-pools providers create-oidc external-jwt-provider-$NAME \
  --location=global \
  --workload-identity-pool=$NAME \
  --issuer-uri="https://my-external-idp.example.com" \
  --allowed-audiences="gcp-workload-identity" \
  --attribute-mapping="google.subject=assertion.sub" \
  --jwk-json-path=<(echo "$JWKS_CONTENT") \
  --project $PROJECT_ID

# Create service account
echo "Creating Service Account..."
print_command "gcloud iam service-accounts create $NAME"
gcloud iam service-accounts create $NAME
export SA_EMAIL="${NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
echo "Service Account Email: $SA_EMAIL"

# Grant permissions
echo "Granting permissions to Service Account..."
print_command "gcloud projects add-iam-policy-binding $PROJECT_ID --member=\"serviceAccount:${SA_EMAIL}\" --role=\"roles/pubsub.viewer\" --condition=None"
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/pubsub.viewer" --condition=None

echo "Binding Workload Identity Pool to Service Account..."
print_command "gcloud iam service-accounts add-iam-policy-binding $SA_EMAIL --member=\"principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${NAME}/*\" --role=\"roles/iam.workloadIdentityUser\" --condition=None"
gcloud iam service-accounts add-iam-policy-binding $SA_EMAIL \
  --member="principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${NAME}/*" \
  --role="roles/iam.workloadIdentityUser" --condition=None

# gcloud iam service-accounts add-iam-policy-binding $SA_EMAIL \
# --member="principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${NAME}/*" \
# --role="roles/iam.serviceAccountTokenCreator" --condition=None

echo "Creating Pub/Sub topic..."
print_command "gcloud pubsub topics create $NAME --project $PROJECT_ID"
gcloud pubsub topics create $NAME --project $PROJECT_ID

echo ""
print_header "========================================="
print_header "Creating JWT..."
print_header "========================================="
print_command "go run cmd/create-jwt/main.go --key-id $KEY_ID --issuer https://my-external-idp.example.com --audience gcp-workload-identity --subject external-user-123 --email user@example.com --environment production --private-key $PRIVATE_KEY_FILE --output $EXTERNAL_TOKEN_FILE"
echo "-----------------------------------------"
go run cmd/create-jwt/main.go \
  --key-id "$KEY_ID" \
  --issuer https://my-external-idp.example.com \
  --audience gcp-workload-identity \
  --subject external-user-123 \
  --email user@example.com \
  --environment production \
  --private-key "$PRIVATE_KEY_FILE" \
  --output "$EXTERNAL_TOKEN_FILE"
echo ""

print_header "========================================="
print_header "Exchanging token..."
print_header "========================================="
print_command "go run cmd/exchange-token/main.go --project-number $PROJECT_NUMBER --pool-id $NAME --provider-id external-jwt-provider-$NAME --service-account $SA_EMAIL --token-input $EXTERNAL_TOKEN_FILE --output $GCP_ACCESS_TOKEN_FILE"
echo "-----------------------------------------"
echo "Note: This step may fail initially while permissions propagate through GCP."
echo "Will retry every 10 seconds until successful..."
echo ""

# Retry loop for exchange-token
MAX_RETRIES=30
RETRY_COUNT=0
RETRY_DELAY=10

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
  RETRY_COUNT=$((RETRY_COUNT + 1))

  if [ $RETRY_COUNT -gt 1 ]; then
    echo "Retry attempt $RETRY_COUNT of $MAX_RETRIES..."
  fi

  # Remove any existing partial output file
  rm -f "$GCP_ACCESS_TOKEN_FILE"

  # Try to exchange the token
  if go run cmd/exchange-token/main.go \
    --project-number $PROJECT_NUMBER \
    --pool-id $NAME \
    --provider-id external-jwt-provider-$NAME \
    --service-account $SA_EMAIL \
    --token-input "$EXTERNAL_TOKEN_FILE" \
    --output "$GCP_ACCESS_TOKEN_FILE" 2>&1; then

    # Check if the output file was created and is not empty
    if [ -f "$GCP_ACCESS_TOKEN_FILE" ] && [ -s "$GCP_ACCESS_TOKEN_FILE" ]; then
      echo ""
      echo -e "${COLOR_GREEN}✓ Token exchange successful!${COLOR_RESET}"
      break
    fi
  fi

  if [ $RETRY_COUNT -lt $MAX_RETRIES ]; then
    echo "Token exchange failed. Waiting ${RETRY_DELAY} seconds before retry..."
    echo "This is normal - GCP permissions can take time to propagate."
    sleep $RETRY_DELAY
  else
    echo ""
    echo -e "${COLOR_RESET}Error: Token exchange failed after $MAX_RETRIES attempts."
    echo "Please check:"
    echo "  1. Service account bindings are correct"
    echo "  2. Workload Identity Pool configuration is correct"
    echo "  3. JWT token is valid"
    exit 1
  fi
done
echo ""

print_header "========================================="
print_header "Listing Pub/Sub topics..."
print_header "========================================="
print_command "go run cmd/list-topics/main.go --project-id $PROJECT_ID --token-input $GCP_ACCESS_TOKEN_FILE"
echo "-----------------------------------------"
go run cmd/list-topics/main.go --project-id $PROJECT_ID --token-input "$GCP_ACCESS_TOKEN_FILE"
echo ""

echo ""
print_header "========================================="
echo -e "${COLOR_GREEN}SUCCESS! All steps completed.${COLOR_RESET}"
print_header "========================================="
echo ""
echo "Configuration used:"
echo "  Project ID: $PROJECT_ID"
echo "  Project Number: $PROJECT_NUMBER"
echo "  Pool ID: $NAME"
echo "  Provider ID: external-jwt-provider-$NAME"
echo "  Service Account: $SA_EMAIL"
echo "  Key ID: $KEY_ID"
echo ""
echo "File paths used:"
echo "  Private Key: $PRIVATE_KEY_FILE"
echo "  Public Key: $PUBLIC_KEY_FILE"
echo "  JWK: $JWK_FILE"
echo "  JWKS: $JWKS_FILE"
echo "  External Token: $EXTERNAL_TOKEN_FILE"
echo "  GCP Access Token: $GCP_ACCESS_TOKEN_FILE"
echo ""
print_header "========================================="
print_header "Cleanup commands (not executed):"
print_header "========================================="
echo ""
echo "# Delete Pub/Sub topic"
print_command "gcloud pubsub topics delete $NAME --project $PROJECT_ID"
echo ""
echo "# Delete service account"
print_command "gcloud iam service-accounts delete $SA_EMAIL --project $PROJECT_ID"
echo ""
echo "# Delete Workload Identity Provider"
print_command "gcloud iam workload-identity-pools providers delete external-jwt-provider-$NAME --workload-identity-pool=$NAME --location=global --project $PROJECT_ID"
echo ""
echo "# Delete Workload Identity Pool"
print_command "gcloud iam workload-identity-pools delete $NAME --location=global --project $PROJECT_ID"
echo ""
echo "# Delete project (optional)"
print_command "gcloud projects delete $PROJECT_ID"
echo ""
echo "# Clean up local files"
print_command "make clean"
