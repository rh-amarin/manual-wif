#!/bin/bash

# Check if project_id and pool_name parameters are provided
if [ -z "$1" ] || [ -z "$2" ]; then
  echo "Error: project_id and pool_name parameters are required"
  echo "Usage: $0 <project_id> <pool_name>"
  exit 1
fi

export PROJECT_ID="$1"
export POOL_NAME="$2"

gcloud projects create $PROJECT_ID

gcloud config set project $PROJECT_ID


export PROJECT_NUMBER=$(gcloud projects describe $PROJECT_ID --format="value(projectNumber)")
echo "Project ID: $PROJECT_ID"
echo "Project Number: $PROJECT_NUMBER"

# Enable APIs
gcloud services enable iamcredentials.googleapis.com sts.googleapis.com pubsub.googleapis.com --project $PROJECT_ID

# Create pool
echo "Creating Workload Identity Pool..."
eval gcloud iam workload-identity-pools create $POOL_NAME --location=global --project $PROJECT_ID 

echo "Workload Identity Pool created."

echo "Generating keys..."
go run step1_generate_keys.go

echo "Generating JWK..."
go run step1b_generate_jwk.go

echo "Creating Workload Identity Provider..."
# Create provider with inline JWK (after running step1 and step1b to generate public_key.jwks.json)
export JWKS_CONTENT=$(cat public_key.jwks)
gcloud iam workload-identity-pools providers create-oidc external-jwt-provider \
  --location=global \
  --workload-identity-pool=$POOL_NAME \
  --issuer-uri="https://my-external-idp.example.com" \
  --allowed-audiences="gcp-workload-identity" \
  --attribute-mapping="google.subject=assertion.sub" \
  --jwk-json-path=<(echo "$JWKS_CONTENT") \
  --project $PROJECT_ID

# Create service account
echo "Creating Service Account..."
gcloud iam service-accounts create wif-sa
export SA_EMAIL="wif-sa@${PROJECT_ID}.iam.gserviceaccount.com"
echo "Service Account Email: $SA_EMAIL"

# Grant permissions
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/pubsub.viewer" \
  --condition=None

gcloud iam service-accounts add-iam-policy-binding $SA_EMAIL \
  --member="principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${POOL_NAME}/*" \
  --role="roles/iam.workloadIdentityUser"

echo "Creating Pub/Sub topic..."
gcloud pubsub topics create test-topic-1 --project $PROJECT_ID

echo "creating config.json file..."
cat <<EOF > config.json
{
  "project_id": "$PROJECT_ID",
  "project_number": "$PROJECT_NUMBER",
  "pool_id": "$POOL_NAME",
  "provider_id": "external-jwt-provider",
  "service_account_email": "wif-sa@$PROJECT_ID.iam.gserviceaccount.com",
  "issuer_url": "https://my-external-idp.example.com",
  "jwt_audience": "gcp-workload-identity",
  "key_id": "key-1",
  "subject": "external-user-123",
  "user_email": "user@example.com",
  "environment": "production"
}
EOF

echo "Config file created:"
cat config.json

echo "Running steps to create JWT, exchange token, and list topics..."
go run step2_create_jwt.go

go run step3_exchange_token.go

go run step4_list_topics.go


echo ""
echo "========================================="
echo "Cleanup commands (not executed):"
echo "========================================="
echo ""
echo "# Delete Pub/Sub topic"
echo "gcloud pubsub topics delete test-topic-1 --project $PROJECT_ID"
echo ""
echo "# Delete service account"
echo "gcloud iam service-accounts delete wif-sa@${PROJECT_ID}.iam.gserviceaccount.com --project $PROJECT_ID"
echo ""
echo "# Delete Workload Identity Provider"
echo "gcloud iam workload-identity-pools providers delete external-jwt-provider --workload-identity-pool=$POOL_NAME --location=global --project $PROJECT_ID"
echo ""
echo "# Delete Workload Identity Pool"
echo "gcloud iam workload-identity-pools delete $POOL_NAME --location=global --project $PROJECT_ID"
echo ""
echo "# Delete project (optional)"
echo "gcloud projects delete $PROJECT_ID"
