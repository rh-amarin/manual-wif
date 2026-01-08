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

## Quick Start

### 1. Build the Commands

```bash
make build
```

This will:
- Install dependencies (`github.com/golang-jwt/jwt/v5`)
- Build all command binaries to the `bin/` directory

### 2. Complete GCP Setup

Follow the instructions in [GCP_SETUP.md](GCP_SETUP.md) to:
- Create a Workload Identity Pool
- Create a Workload Identity Provider
- Create and configure a Service Account
- Upload your public JWK

### 3. Run the Commands

Each command will print the exact format needed for the next step.

```bash
# Step 1: Generate RSA key pair
./bin/generate-keys

# Step 2: Generate JWK format (use the key-id you'll configure in GCP)
./bin/generate-jwk --key-id key-1

# Step 3: Create and sign a JWT token
./bin/create-jwt \
  --key-id key-1 \
  --issuer https://my-external-idp.example.com \
  --audience gcp-workload-identity \
  --subject external-user-123 \
  --email user@example.com \
  --environment production

# Step 4: Exchange JWT for GCP access token
./bin/exchange-token \
  --project-number 123456789 \
  --pool-id my-pool \
  --provider-id my-provider \
  --service-account my-sa@my-project.iam.gserviceaccount.com

# Step 5: Use the access token to call GCP APIs
./bin/list-topics --project-id my-project
```

**Important**: Replace the example values with your actual GCP project details.


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

### "Workload identity pool does not exist"
- Run the GCP setup commands in [GCP_SETUP.md](GCP_SETUP.md)
- Verify: `gcloud iam workload-identity-pools describe external-identity-pool --location=global`

### "Invalid JWT"
- Check that `iss` in JWT matches provider's `--issuer-uri`
- Check that `aud` in JWT matches provider's `--allowed-audiences`
- Verify JWT has all required claims: `iss`, `sub`, `aud`, `exp`, `iat`

### "Permission denied" from STS API
- Verify service account has `workloadIdentityUser` role binding
- Check the audience in your token exchange matches your pool/provider

### "Permission denied" from Pub/Sub API
- Verify service account has `roles/pubsub.viewer` on the project
- Ensure the access token is from step 3b (not the federated token from 3a)

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
- `bin/` - Compiled command binaries (created by `make build`)
- `private_key.pem` - RSA private key (keep secret!)
- `public_key.pem` - RSA public key in PEM format
- `public_key.jwk` - RSA public key as single JSON Web Key
- `public_key.jwks` - RSA public key as JSON Web Key Set (for GCP)
- `external_token.jwt` - Signed JWT token
- `gcp_access_token.txt` - GCP access token

## License

This is a learning POC - feel free to use and modify as needed.
