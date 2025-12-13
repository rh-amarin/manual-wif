# Quick Start Guide

Get the POC running in 5 minutes (assuming you have a GCP project ready).

## 1. Install Dependencies

```bash
go mod tidy
```

## 2. Set Up GCP (One-Time)

```bash
# Set your project
export PROJECT_ID="your-project-id"
export PROJECT_NUMBER=$(gcloud projects describe $PROJECT_ID --format="value(projectNumber)")

# Enable required APIs
gcloud services enable iamcredentials.googleapis.com sts.googleapis.com pubsub.googleapis.com

# Create Workload Identity Pool
gcloud iam workload-identity-pools create external-identity-pool \
  --location=global \
  --display-name="External Identity Pool"

# Create Workload Identity Provider
gcloud iam workload-identity-pools providers create-oidc external-jwt-provider \
  --location=global \
  --workload-identity-pool=external-identity-pool \
  --issuer-uri="https://my-external-idp.example.com" \
  --allowed-audiences="gcp-workload-identity" \
  --attribute-mapping="google.subject=assertion.sub"

# Create Service Account
gcloud iam service-accounts create wif-sa \
  --display-name="WIF Service Account"

export SA_EMAIL="wif-sa@${PROJECT_ID}.iam.gserviceaccount.com"

# Grant Pub/Sub permissions
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/pubsub.viewer"

# Allow workload identity pool to impersonate SA
gcloud iam service-accounts add-iam-policy-binding $SA_EMAIL \
  --member="principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/external-identity-pool/*" \
  --role="roles/iam.workloadIdentityUser"

# Create config file
cat > config.json <<EOF
{
  "project_id": "${PROJECT_ID}",
  "project_number": "${PROJECT_NUMBER}",
  "pool_id": "external-identity-pool",
  "provider_id": "external-jwt-provider",
  "service_account_email": "${SA_EMAIL}"
}
EOF

# (Optional) Create a test topic
gcloud pubsub topics create test-topic
```

## 3. Run the POC

### Option A: Run all steps at once
```bash
go run all_steps.go
```

### Option B: Run step by step
```bash
go run step1_generate_keys.go
go run step2_create_jwt.go
go run step3_exchange_token.go
go run step4_list_topics.go
```

### Option C: Use Makefile
```bash
make all
```

## What You Should See

```
╔════════════════════════════════════════════════════════════════╗
║  GCP Workload Identity Federation - Complete Flow             ║
╚════════════════════════════════════════════════════════════════╝

✓ Configuration loaded
  Project ID: your-project-id
  Service Account: wif-sa@your-project-id.iam.gserviceaccount.com

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
STEP 1: Generate RSA Key Pair
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✓ Generated RSA-2048 key pair

...

🎉 SUCCESS! Workload Identity Federation is working!
```

## Troubleshooting

### "config.json: no such file"
Run the setup commands above to create `config.json`.

### "Workload identity pool does not exist"
Run the `gcloud iam workload-identity-pools create` command from step 2.

### "Permission denied"
Make sure you ran all the `gcloud iam service-accounts add-iam-policy-binding` commands.

### "No topics found"
This is OK! The POC still works. Create a test topic:
```bash
gcloud pubsub topics create test-topic --project=$PROJECT_ID
```

## Next Steps

- Read [README.md](README.md) for detailed explanations
- Read [GCP_SETUP.md](GCP_SETUP.md) for production guidance
- Modify JWT claims in `step2_create_jwt.go` and see what happens
- Try adding attribute conditions to the provider
- Explore using this with a real external IdP
