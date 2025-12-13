# GCP Workload Identity Federation Setup Guide

This guide walks through setting up GCP Workload Identity Federation to allow an external JWT token to access GCP resources.

## Prerequisites

- A GCP project with billing enabled
- `gcloud` CLI installed and authenticated
- Permissions to create:
  - Workload Identity Pools
  - Workload Identity Providers
  - Service Accounts
  - IAM bindings

## Overview of What We're Setting Up

```
External Identity (JWT) → Workload Identity Pool/Provider → Service Account → GCP Resources
```

1. **External Identity**: A JWT token signed by your own private key (simulating an external IdP)
2. **Workload Identity Pool**: A container for external identities
3. **Workload Identity Provider**: Defines how to validate external tokens
4. **Service Account**: The GCP identity that will actually access resources
5. **IAM Bindings**: Permissions for the service account and mapping from external identity

## Step-by-Step Setup

### 1. Set Your Project

```bash
export PROJECT_ID="your-gcp-project-id"
gcloud config set project $PROJECT_ID

# Get your project number (you'll need this)
export PROJECT_NUMBER=$(gcloud projects describe $PROJECT_ID --format="value(projectNumber)")
echo "Project Number: $PROJECT_NUMBER"
```

### 2. Enable Required APIs

```bash
gcloud services enable iamcredentials.googleapis.com
gcloud services enable sts.googleapis.com
gcloud services enable pubsub.googleapis.com
```

### 3. Create a Workload Identity Pool

The pool is a container for external identities.

```bash
gcloud iam workload-identity-pools create external-identity-pool \
  --location=global \
  --display-name="External Identity Pool" \
  --description="Pool for external JWT tokens"
```

Verify:
```bash
gcloud iam workload-identity-pools describe external-identity-pool \
  --location=global
```

### 4. Upload Your Public Key

First, generate the keys by running the Go script:

```bash
go run step1_generate_keys.go
```

This creates `public_key.pem` and `private_key.pem`.

Convert the public key to JWK format (JSON Web Key) that GCP expects:

```bash
# You'll need to manually create a JWK from your public key
# Here's a simple way using openssl and manual formatting:

# Get the modulus and exponent
openssl rsa -pubin -in public_key.pem -text -noout
```

**Note**: For this POC, we'll use a simpler approach - we'll configure the provider to skip signature verification (insecure, but simpler for learning). In production, you would:
1. Convert the PEM to JWK format
2. Host the JWK at a public JWKS URI
3. Configure the provider to use that URI

### 5. Create a Workload Identity Provider

This tells GCP how to validate external tokens.

```bash
gcloud iam workload-identity-pools providers create-oidc external-jwt-provider \
  --location=global \
  --workload-identity-pool=external-identity-pool \
  --issuer-uri="https://my-external-idp.example.com" \
  --allowed-audiences="gcp-workload-identity" \
  --attribute-mapping="google.subject=assertion.sub,attribute.email=assertion.email" \
  --attribute-condition="assertion.aud == 'gcp-workload-identity'"
```

**Explanation of parameters:**
- `--issuer-uri`: Must match the `iss` claim in your JWT
- `--allowed-audiences`: Must match the `aud` claim in your JWT
- `--attribute-mapping`: Maps JWT claims to GCP attributes
  - `google.subject` is required and becomes the principal identifier
  - `attribute.*` can be used for additional claims
- `--attribute-condition`: Additional validation (optional)

For this POC without a public JWKS endpoint, use a different approach:

```bash
# Create provider without JWKS (allows any signature - NOT SECURE!)
gcloud iam workload-identity-pools providers create-oidc external-jwt-provider \
  --location=global \
  --workload-identity-pool=external-identity-pool \
  --issuer-uri="https://my-external-idp.example.com" \
  --allowed-audiences="gcp-workload-identity" \
  --attribute-mapping="google.subject=assertion.sub"
```

**IMPORTANT**: Without JWKS verification, GCP won't validate the JWT signature. This is only OK for learning/testing!

### 6. Create a Service Account

This is the GCP identity that will access resources.

```bash
gcloud iam service-accounts create wif-sa \
  --display-name="Workload Identity Federation Service Account"

export SA_EMAIL="wif-sa@${PROJECT_ID}.iam.gserviceaccount.com"
echo "Service Account: $SA_EMAIL"
```

### 7. Grant Pub/Sub Permissions to Service Account

```bash
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/pubsub.viewer"
```

### 8. Allow External Identity to Impersonate Service Account

This is the key step that links the external identity to the GCP service account.

```bash
gcloud iam service-accounts add-iam-policy-binding $SA_EMAIL \
  --member="principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/external-identity-pool/*" \
  --role="roles/iam.workloadIdentityUser"
```

**Explanation:**
- `principalSet://.../*`: Allows ANY identity from the pool to impersonate this SA
- In production, you'd restrict this to specific subjects:
  ```bash
  --member="principal://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/external-identity-pool/subject/external-user-123"
  ```

### 9. Create Configuration File

Create `config.json` with your settings:

```bash
cat > config.json <<EOF
{
  "project_id": "${PROJECT_ID}",
  "project_number": "${PROJECT_NUMBER}",
  "pool_id": "external-identity-pool",
  "provider_id": "external-jwt-provider",
  "service_account_email": "${SA_EMAIL}"
}
EOF
```

### 10. (Optional) Create a Test Pub/Sub Topic

```bash
gcloud pubsub topics create test-topic
```

## Verification Commands

Check your setup:

```bash
# List pools
gcloud iam workload-identity-pools list --location=global

# List providers
gcloud iam workload-identity-pools providers list \
  --workload-identity-pool=external-identity-pool \
  --location=global

# Check service account IAM bindings
gcloud iam service-accounts get-iam-policy $SA_EMAIL
```

## What Each Component Does

### Workload Identity Pool
- Namespace for external identities
- Isolates different sets of external identities

### Workload Identity Provider
- Validates external tokens
- Maps external identity attributes to GCP attributes
- In production, would verify JWT signatures using JWKS

### Service Account
- Actual GCP identity with permissions
- Can be granted IAM roles on GCP resources

### IAM Binding
- Links external identity (from pool) to service account
- Uses `workloadIdentityUser` role for impersonation

## Security Considerations

⚠️ **This POC is NOT production-ready because:**

1. **No signature verification**: Without configuring JWKS, GCP can't verify JWT signatures
2. **Broad principal mapping**: Using `principalSet/*` allows any identity from the pool
3. **Self-signed keys**: Production would use proper PKI or existing IdP

**For production:**
- Host JWKS at a public, secure endpoint
- Configure the provider with `--jwk-json-path` pointing to your JWKS
- Use specific principal mappings based on `subject` or other claims
- Implement proper key rotation
- Add attribute conditions for additional security

## Troubleshooting

### "Workload identity pool does not exist"
Check the pool exists:
```bash
gcloud iam workload-identity-pools describe external-identity-pool --location=global
```

### "Permission denied" when exchanging tokens
- Verify the issuer and audience in your JWT match the provider configuration
- Check the service account IAM binding includes the workload identity pool

### "Invalid JWT"
- Ensure your JWT has all required claims: `iss`, `sub`, `aud`, `exp`, `iat`
- Verify the `iss` matches the provider's `--issuer-uri`
- Verify the `aud` is in the provider's `--allowed-audiences`

## Next Steps

Once setup is complete, run the Go scripts in order:
1. `go run step1_generate_keys.go` (already done for config)
2. `go run step2_create_jwt.go`
3. `go run step3_exchange_token.go`
4. `go run step4_list_topics.go`
