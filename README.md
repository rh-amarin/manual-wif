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
├── config.json.template        # Configuration template
├── config.json                 # Your configuration (create from template)
│
├── step1_generate_keys.go      # Generate RSA key pair
├── step1b_generate_jwk.go      # Generate public JWK file to upload to GCP
├── step2_create_jwt.go         # Create and sign JWT token
├── step3_exchange_token.go     # Exchange JWT for GCP access token
└── step4_list_topics.go        # Use access token to call Pub/Sub API
```

## Prerequisites

1. **Go**: Version 1.19 or later
2. **GCP Project**: With billing enabled
3. **gcloud CLI**: Installed and authenticated
4. **Permissions**: To create IAM resources in GCP

## Quick Start

### 1. Install Dependencies

```bash
go mod tidy
```

This will download:
- `github.com/golang-jwt/jwt/v5` - For JWT creation and signing

Execute:

```bash
./execute_all.sh "your-gcp-project-id" "name-of-identity-pool"
```

If the project/pool exists the script will give some errors but will keep running


## What Each Step Does

### Step 1: Generate Keys
- Creates an RSA-2048 key pair
- **private_key.pem**: Used to sign JWTs (keep secret!)
- **public_key.pem**: Public key in PEM format

**Key concept**: In a real scenario, this would be your external identity provider's signing key.

### Step 1b: Generate JWK
- Converts the public key from PEM to JWK (JSON Web Key) format
- **public_key.jwk**: Single JSON Web Key
- **public_key.jwks.json**: JSON Web Key Set (what GCP expects)

**Key concept**: GCP needs the public key in JWK format to verify JWT signatures. You can either host this at a public URL or provide it inline when configuring the Workload Identity Provider. See [JWK_UPLOAD_GUIDE.md](JWK_UPLOAD_GUIDE.md) for details.

### Step 2: Create JWT
- Constructs a JWT with claims identifying the external user
- Signs it with the private key
- **Claims explained**:
  - `iss` (issuer): Identifies your external IdP - must match GCP provider config
  - `sub` (subject): The user/identity - becomes `google.subject` in GCP
  - `aud` (audience): Who should accept this token - must match GCP provider config
  - `exp` (expiration): When the token expires
  - `iat` (issued at): When the token was created

**Key concept**: This JWT proves "I am user X from external system Y"

### Step 3: Exchange Token
This is a **two-step exchange**:

**Step 3a**: External JWT → Federated Token
- Calls GCP STS API with your JWT
- GCP validates the JWT (issuer, audience, expiration)
- Returns a federated token (short-lived, can't do much yet)

**Step 3b**: Federated Token → Access Token
- Calls GCP STS API again
- Exchanges federated token for an access token
- This step "impersonates" the service account
- Returns a full GCP access token with the service account's permissions

**Key concept**: Two exchanges provide security boundaries - first validates external identity, second grants GCP permissions.

### Step 4: Call GCP API
- Uses the access token to authenticate to GCP Pub/Sub API
- Lists topics in the project
- Demonstrates the external identity successfully accessing GCP resources

**Key concept**: The access token works exactly like a token from `gcloud auth print-access-token`

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
- `private_key.pem` - RSA private key (keep secret!)
- `public_key.pem` - RSA public key in PEM format
- `public_key.jwk` - RSA public key as single JSON Web Key
- `public_key.jwks.json` - RSA public key as JSON Web Key Set (for GCP)
- `external_token.jwt` - Signed JWT token
- `gcp_access_token.txt` - GCP access token
- `config.json` - Your configuration

## License

This is a learning POC - feel free to use and modify as needed.
