#!/bin/bash

# Check if project_id and name parameters are provided
if [ -z "$1" ] || [ -z "$2" ]; then
  echo "Error: project_id and name parameters are required"
  echo "Usage: $0 <project_id> <name>"
  exit 1
fi

export PROJECT_ID="$1"
export NAME="$2"

# Ensure NAME is at least 6 characters
if [ ${#NAME} -lt 6 ]; then
  echo "Error: name parameter must be at least 6 characters"
  echo "Usage: $0 <project_id> <name>"
  exit 1
fi

gcloud projects create $PROJECT_ID

gcloud config set project $PROJECT_ID


export PROJECT_NUMBER=$(gcloud projects describe $PROJECT_ID --format="value(projectNumber)")
echo "Project ID: $PROJECT_ID"
echo "Project Number: $PROJECT_NUMBER"

# Enable APIs
gcloud services enable iamcredentials.googleapis.com sts.googleapis.com pubsub.googleapis.com --project $PROJECT_ID

# Create pool
echo "Creating Workload Identity Pool..."
eval gcloud iam workload-identity-pools create $NAME --location=global --project $PROJECT_ID 

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
  --workload-identity-pool=$NAME \
  --issuer-uri="https://my-external-idp.example.com" \
  --allowed-audiences="gcp-workload-identity" \
  --attribute-mapping="google.subject=assertion.sub" \
  --jwk-json-path=<(echo "$JWKS_CONTENT") \
  --project $PROJECT_ID

# Create service account
echo "Creating Service Account..."
gcloud iam service-accounts create $NAME
export SA_EMAIL="${NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
echo "Service Account Email: $SA_EMAIL"

# Grant permissions
echo "Granting permissions to Service Account..."
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/pubsub.viewer" 

echo "Binding Workload Identity Pool to Service Account..."
gcloud iam service-accounts add-iam-policy-binding $SA_EMAIL \
  --member="principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${NAME}/*" \
  --role="roles/iam.workloadIdentityUser"

gcloud iam service-accounts add-iam-policy-binding $SA_EMAIL \
  --member="principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${NAME}/*" \
  --role="roles/iam.serviceAccountTokenCreator"


echo "Creating Pub/Sub topic..."
gcloud pubsub topics create $NAME --project $PROJECT_ID

echo "creating config.json file..."
cat <<EOF > config.json
{
  "project_id": "$PROJECT_ID",
  "project_number": "$PROJECT_NUMBER",
  "pool_id": "$NAME",
  "provider_id": "external-jwt-provider-$NAME",
  "service_account_email": "$SA_EMAIL",
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
echo "gcloud pubsub topics delete $NAME --project $PROJECT_ID"
echo ""
echo "# Delete service account"
echo "gcloud iam service-accounts delete $SA_EMAIL --project $PROJECT_ID"
echo ""
echo "# Delete Workload Identity Provider"
echo "gcloud iam workload-identity-pools providers delete external-jwt-provider --workload-identity-pool=$NAME --location=global --project $PROJECT_ID"
echo ""
echo "# Delete Workload Identity Pool"
echo "gcloud iam workload-identity-pools delete $NAME --location=global --project $PROJECT_ID"
echo ""
echo "# Delete project (optional)"
echo "gcloud projects delete $PROJECT_ID"

