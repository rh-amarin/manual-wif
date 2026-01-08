# GCP Workload Identity Federation POC

A hands-on proof of concept to understand GCP Workload Identity Federation by implementing each step manually using basic Go tools.

## What This POC Demonstrates

This project shows how an **external identity** (outside of GCP) can access GCP resources through **Workload Identity Federation**, without using service account keys.

### The Flow

```
┌─────────────────────┐
│  External Identity  │  (Your system/application)
│  Creates JWT Token  │
└──────────┬──────────┘
           │ 1. JWT signed with private key
           ▼
┌─────────────────────┐
│   GCP STS API       │
│  Token Exchange     │
└──────────┬──────────┘
           │ 2. Validates JWT (issuer, audience, claims)
           │ 3. Returns federated token
           ▼
┌─────────────────────┐
│   GCP STS API       │
│  Second Exchange    │
└──────────┬──────────┘
           │ 4. Exchanges federated token for access token
           │ 5. Returns GCP access token (with SA permissions)
           ▼
┌─────────────────────┐
│   GCP Pub/Sub API   │
│   List Topics       │
└─────────────────────┘
```

## Why This Matters

**Traditional approach**: Download a service account key (JSON file) → security risk if leaked

**Workload Identity Federation**: External system proves its identity → GCP grants temporary access → no long-lived credentials

**Use cases:**
- AWS EC2 accessing GCP resources
- GitHub Actions deploying to GCP
- On-premise systems accessing GCP
- Multi-cloud architectures

## Project Structure

```
.
├── README.md                    # This file
├── GCP_SETUP.md                # Detailed GCP configuration guide
├── Makefile                    # Build commands
├── execute_all.sh              # Automated script to run complete flow
│
├── cmd/
│   ├── generate-keys/          # Generate RSA key pair
│   ├── generate-jwk/           # Generate public JWK file to upload to GCP
│   ├── create-jwt/             # Create and sign JWT token
│   ├── exchange-token/         # Exchange JWT for GCP access token
│   └── list-topics/            # Use access token to call Pub/Sub API
│
└── bin/                        # Compiled binaries (after make build)
```

## Prerequisites

1. **Go**: Version 1.19 or later
2. **GCP Project**: With billing enabled
3. **gcloud CLI**: Installed and authenticated
4. **Permissions**: To create IAM resources in GCP

## Quick Start: Automated Setup (Recommended for Testing)

Run the entire flow with a single script:

```bash
./execute_all.sh <project_id> <name>
```

**Parameters:**
- `project_id`: Your GCP project ID (will be created if it doesn't exist)
- `name`: A unique identifier (minimum 6 characters) used for naming resources

**Example:**
```bash
./execute_all.sh my-wif-test mytest01
```
Note: step 10 can take some time for the oidc-provider to properly initialize. (It retries in case of errors)

**What the script does:**
1. Creates a new GCP project (or uses existing)
2. Enables required APIs (IAM, STS, Pub/Sub)
3. Creates Workload Identity Pool
4. Generates RSA key pair and JWK files
5. Creates Workload Identity Provider with inline JWK
6. Creates and configures Service Account
7. Grants necessary permissions
8. Creates a test Pub/Sub topic
9. Generates and signs a JWT token
10. Exchanges JWT for GCP access token (with automatic retries)
11. Uses the access token to list Pub/Sub topics

**Note:** The token exchange step may take 1-5 minutes as GCP permissions propagate. The script automatically retries every 10 seconds until successful.

**Verification:**
If successful, you'll see a list of topics in the project

The script will display cleanup commands at the end to delete all created resources.

**Files created:**
- `private_key_<name>.pem` - Private signing key
- `public_key_<name>.pem` - Public key (PEM format)
- `public_key_<name>.jwk` - Public key (JWK format)
- `public_key_<name>.jwks` - Public key set (JWKS format)
- `external_token_<name>.jwt` - Signed JWT token
- `gcp_access_token_<name>.txt` - GCP access token

## Verifying the Setup

After running the automated script or completing the manual steps, you can verify each component:

### 1. Check Workload Identity Pool

```bash
gcloud iam workload-identity-pools describe <name> \
  --location=global \
  --project=<project_id>
```

### 2. Check Workload Identity Provider

```bash
gcloud iam workload-identity-pools providers describe external-jwt-provider-<name> \
  --workload-identity-pool=<name> \
  --location=global \
  --project=<project_id>
```

Verify the output shows:
- `issuerUri: https://my-external-idp.example.com`
- `allowedAudiences: gcp-workload-identity`
- `attributeMapping` includes `google.subject=assertion.sub`

### 3. Check Service Account and Permissions

```bash
# Verify service account exists
gcloud iam service-accounts describe <name>@<project_id>.iam.gserviceaccount.com

# Check Pub/Sub viewer role
gcloud projects get-iam-policy <project_id> \
  --flatten="bindings[].members" \
  --filter="bindings.members:serviceAccount:<name>@<project_id>.iam.gserviceaccount.com AND bindings.role:roles/pubsub.viewer"

# Check workloadIdentityUser binding
gcloud iam service-accounts get-iam-policy <name>@<project_id>.iam.gserviceaccount.com \
  --format=json
```

The last command should show a binding with:
- `role: roles/iam.workloadIdentityUser`
- `members` including `principalSet://iam.googleapis.com/projects/<project_number>/locations/global/workloadIdentityPools/<name>/*`

### 4. Inspect Generated Files

```bash
# View the JWT token claims (without verification)
cat external_token_<name>.jwt | cut -d. -f2 | base64 -d 2>/dev/null | jq .

# View the access token (first 50 chars)
head -c 50 gcp_access_token_<name>.txt && echo "..."
```

The JWT should show claims like:
```json
{
  "iss": "https://my-external-idp.example.com",
  "sub": "external-user-123",
  "aud": "gcp-workload-identity",
  "email": "user@example.com",
  "environment": "production",
  "exp": ...,
  "iat": ...
}
```

### 5. Test the Access Token Manually

```bash
# Use the access token to call Pub/Sub API
curl -H "Authorization: Bearer $(cat gcp_access_token_<name>.txt)" \
  "https://pubsub.googleapis.com/v1/projects/<project_id>/topics"
```

You should see a list of topics including the one created by the script.

### 6. Check Pub/Sub Topic

```bash
gcloud pubsub topics list --project=<project_id>
```

## What Each Step Does

### Step 1: Generate Keys (`./bin/generate-keys`)
- Creates an RSA-2048 key pair
- **private_key.pem**: Used to sign JWTs (keep secret!)
- **public_key.pem**: Public key in PEM format
- **No parameters required**

**Key concept**: In a real scenario, this would be your external identity provider's signing key.

**Output**: Prints the command format for the next step.

### Step 2: Generate JWK (`./bin/generate-jwk`)
- Converts the public key from PEM to JWK (JSON Web Key) format
- **public_key.jwk**: Single JSON Web Key
- **public_key.jwks**: JSON Web Key Set (what GCP expects)

**Parameters**:
- `--key-id`: Key identifier (must match what you configure in GCP)

**Key concept**: GCP needs the public key in JWK format to verify JWT signatures. You can either host this at a public URL or provide it inline when configuring the Workload Identity Provider. See [JWK_UPLOAD_GUIDE.md](JWK_UPLOAD_GUIDE.md) for details.

**Output**: Prints the command format for the next step with the key-id you provided.

### Step 3: Create JWT (`./bin/create-jwt`)
- Constructs a JWT with claims identifying the external user
- Signs it with the private key

**Parameters**:
- `--key-id`: Key identifier (required)
- `--issuer`: Issuer URL (required) - must match GCP provider config
- `--audience`: JWT audience (required) - must match GCP provider config
- `--subject`: Subject/user identifier (required)
- `--email`: User email (optional)
- `--environment`: Environment name (optional)

**Claims explained**:
- `iss` (issuer): Identifies your external IdP - must match GCP provider config
- `sub` (subject): The user/identity - becomes `google.subject` in GCP
- `aud` (audience): Who should accept this token - must match GCP provider config
- `exp` (expiration): When the token expires
- `iat` (issued at): When the token was created

**Key concept**: This JWT proves "I am user X from external system Y"

**Output**: Prints the command format for the next step.

### Step 4: Exchange Token (`./bin/exchange-token`)
This is a **two-step exchange**:

**Step 4a**: External JWT → Federated Token
- Calls GCP STS API with your JWT
- GCP validates the JWT (issuer, audience, expiration)
- Returns a federated token (short-lived, can't do much yet)

**Step 4b**: Federated Token → Access Token
- Calls GCP STS API again
- Exchanges federated token for an access token
- This step "impersonates" the service account
- Returns a full GCP access token with the service account's permissions

**Parameters**:
- `--project-number`: GCP project number (required, not project ID)
- `--pool-id`: Workload Identity Pool ID (required)
- `--provider-id`: Workload Identity Provider ID (required)
- `--service-account`: Service account email to impersonate (required)

**Key concept**: Two exchanges provide security boundaries - first validates external identity, second grants GCP permissions.

**Output**: Prints the command format for the next step.

### Step 5: Call GCP API (`./bin/list-topics`)
- Uses the access token to authenticate to GCP Pub/Sub API
- Lists topics in the project
- Demonstrates the external identity successfully accessing GCP resources

**Parameters**:
- `--project-id`: GCP project ID (required)

**Key concept**: The access token works exactly like a token from `gcloud auth print-access-token`

**Output**: Confirms successful completion of the entire flow.

## Understanding the Token Exchange

The STS (Security Token Service) endpoint is the core of WIF:

```
POST https://sts.googleapis.com/v1/token
Content-Type: application/x-www-form-urlencoded

grant_type=urn:ietf:params:oauth:grant-type:token-exchange
audience=//iam.googleapis.com/projects/PROJECT_NUM/locations/global/workloadIdentityPools/POOL_ID/providers/PROVIDER_ID
subject_token=<your JWT>
subject_token_type=urn:ietf:params:oauth:token-type:jwt
requested_token_type=urn:ietf:params:oauth:token-type:access_token
scope=https://www.googleapis.com/auth/cloud-platform
```

**Parameters explained:**
- `grant_type`: Specifies OAuth 2.0 token exchange
- `audience`: Identifies your WIF pool/provider OR service account
- `subject_token`: The token you're presenting (JWT or federated token)
- `subject_token_type`: What type of token you're presenting
- `requested_token_type`: What you want back
- `scope`: What permissions you want

## Security Notes

⚠️ **This POC prioritizes learning over security:**

1. **No signature verification**: The GCP provider doesn't verify JWT signatures because we don't host a JWKS endpoint
2. **Broad permissions**: `principalSet/*` allows ANY identity from the pool
3. **Self-signed keys**: Not using a proper PKI infrastructure

**For production:**
- Host your public key as JWKS at a public HTTPS endpoint
- Configure the provider with `--jwk-json-path`
- Use specific principal bindings
- Implement key rotation
- Add attribute conditions for defense in depth

See [GCP_SETUP.md](GCP_SETUP.md) for production recommendations.

## Troubleshooting

### Automated Script Issues

#### "Error: name parameter must be at least 6 characters"
The `name` parameter is used for multiple GCP resources that have minimum length requirements. Use a name with at least 6 characters.

#### "Token exchange failed after 30 attempts"
This usually means:
1. **Permissions haven't propagated** - GCP can take 5+ minutes to propagate IAM permissions. The script retries for up to 5 minutes, but you may need to wait longer and run the exchange-token command manually.
2. **Service account binding is incorrect** - Verify with:
   ```bash
   gcloud iam service-accounts get-iam-policy <name>@<project_id>.iam.gserviceaccount.com
   ```
3. **JWT configuration mismatch** - Ensure issuer and audience in the JWT match the provider configuration.

#### "API [service] not enabled on project"
The script enables required APIs automatically, but this can fail if:
- Billing is not enabled on the project
- You don't have permission to enable APIs
- Solution: Manually enable the API and re-run the script:
  ```bash
  gcloud services enable [service-name] --project=<project_id>
  ```

#### Script fails partway through
The script is designed to be somewhat idempotent, but if it fails partway:
1. Check which resources were created using the verification commands above
2. Either delete the partial resources and start fresh, or manually complete the remaining steps
3. Use the cleanup commands printed at the end to remove resources

### Manual Setup Issues

#### "Workload identity pool does not exist"
- Run the GCP setup commands in [GCP_SETUP.md](GCP_SETUP.md)
- Verify: `gcloud iam workload-identity-pools describe <pool-name> --location=global`

#### "Invalid JWT"
- Check that `iss` in JWT matches provider's `--issuer-uri`
- Check that `aud` in JWT matches provider's `--allowed-audiences`
- Verify JWT has all required claims: `iss`, `sub`, `aud`, `exp`, `iat`
- Inspect your JWT claims: `cat external_token_<name>.jwt | cut -d. -f2 | base64 -d 2>/dev/null | jq .`

#### "Permission denied" from STS API
- Verify service account has `workloadIdentityUser` role binding
- Check the audience in your token exchange matches your pool/provider
- Ensure the principal set in the IAM binding matches your pool path

#### "Permission denied" from Pub/Sub API
- Verify service account has `roles/pubsub.viewer` on the project
- Ensure the access token is from the second exchange (not the federated token from the first exchange)
- Check that you're using the correct project ID

## Learning Resources

- [GCP Workload Identity Federation Docs](https://cloud.google.com/iam/docs/workload-identity-federation)
- [OAuth 2.0 Token Exchange RFC](https://datatracker.ietf.org/doc/html/rfc8693)
- [JWT RFC](https://datatracker.ietf.org/doc/html/rfc7519)
- [GCP STS API Reference](https://cloud.google.com/iam/docs/reference/sts/rest)

## Next Steps

Once you understand the basics:

1. **Try different attribute mappings**: Map email, groups, or custom claims
2. **Add attribute conditions**: Restrict access based on JWT claims
3. **Set up proper JWKS**: Host your public key at an HTTPS endpoint
4. **Try with real IdPs**: Configure AWS, Azure AD, or Okta as the provider
5. **Explore other GCP services**: Use the access token with different APIs

## Files Generated

These files are created during execution (gitignored):

### Build Output
- `bin/` - Compiled command binaries (created by `make build`)

### Automated Script Output (when using execute_all.sh)
- `private_key_<name>.pem` - RSA private key (keep secret!)
- `public_key_<name>.pem` - RSA public key in PEM format
- `public_key_<name>.jwk` - RSA public key as single JSON Web Key
- `public_key_<name>.jwks` - RSA public key as JSON Web Key Set (for GCP)
- `external_token_<name>.jwt` - Signed JWT token
- `gcp_access_token_<name>.txt` - GCP access token

### Manual Setup Output (default file names)
- `private_key.pem` - RSA private key (keep secret!)
- `public_key.pem` - RSA public key in PEM format
- `public_key.jwk` - RSA public key as single JSON Web Key
- `public_key.jwks` - RSA public key as JSON Web Key Set (for GCP)
- `external_token.jwt` - Signed JWT token
- `gcp_access_token.txt` - GCP access token

**Note:** The automated script uses the `<name>` parameter in filenames to allow running multiple tests in parallel without conflicts.

## License

This is a learning POC - feel free to use and modify as needed.
