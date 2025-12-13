# Technical Deep Dive

This document explains the technical details of how Workload Identity Federation works at a deeper level.

## JWT Structure

### Header
```json
{
  "alg": "RS256",
  "typ": "JWT"
}
```
- `alg`: Signature algorithm - RS256 means RSA with SHA-256
- `typ`: Token type - always "JWT"

### Payload (Claims)
```json
{
  "iss": "https://my-external-idp.example.com",
  "sub": "external-user-123",
  "aud": "gcp-workload-identity",
  "iat": 1234567890,
  "exp": 1234571490,
  "email": "user@example.com"
}
```

**Standard Claims:**
- `iss` (issuer): URL identifying who created the token
- `sub` (subject): Unique identifier for the user/identity
- `aud` (audience): Who should accept this token
- `iat` (issued at): Unix timestamp when token was created
- `exp` (expiration): Unix timestamp when token expires

**Custom Claims:**
- Any additional claims you want to include
- Can be used in attribute mappings and conditions

### Signature
The signature is created by:
1. Base64URL encoding the header
2. Base64URL encoding the payload
3. Concatenating with a period: `header.payload`
4. Signing this string with RSA-SHA256 using the private key
5. Base64URL encoding the signature

**Final JWT format:**
```
<base64url(header)>.<base64url(payload)>.<base64url(signature)>
```

## Token Exchange Flow (Detailed)

### Exchange 1: JWT → Federated Token

**Request:**
```http
POST https://sts.googleapis.com/v1/token HTTP/1.1
Content-Type: application/x-www-form-urlencoded

grant_type=urn:ietf:params:oauth:grant-type:token-exchange&
audience=//iam.googleapis.com/projects/PROJECT_NUM/locations/global/workloadIdentityPools/POOL_ID/providers/PROVIDER_ID&
subject_token=eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...&
subject_token_type=urn:ietf:params:oauth:token-type:jwt&
requested_token_type=urn:ietf:params:oauth:token-type:access_token&
scope=https://www.googleapis.com/auth/cloud-platform
```

**What GCP Does:**
1. Parses the JWT
2. Validates JWT structure (header, payload, signature parts exist)
3. Checks expiration (`exp` claim)
4. Verifies issuer matches provider's `--issuer-uri`
5. Verifies audience matches provider's `--allowed-audiences`
6. (In production) Verifies signature using JWKS
7. Applies attribute mappings to create GCP principal
8. Evaluates attribute conditions
9. Issues federated token

**Response:**
```json
{
  "access_token": "ya29.federated-token-here...",
  "issued_token_type": "urn:ietf:params:oauth:token-type:access_token",
  "token_type": "Bearer",
  "expires_in": 3600
}
```

**Federated Token Characteristics:**
- Short-lived (typically 1 hour)
- Can't directly access GCP resources
- Can only be used for service account impersonation
- Contains the mapped GCP principal

### Exchange 2: Federated Token → Access Token

**Request:**
```http
POST https://sts.googleapis.com/v1/token HTTP/1.1
Content-Type: application/x-www-form-urlencoded

grant_type=urn:ietf:params:oauth:grant-type:token-exchange&
audience=//iam.googleapis.com/projects/PROJECT_NUM/serviceAccounts/SA_EMAIL&
subject_token=ya29.federated-token-here...&
subject_token_type=urn:ietf:params:oauth:token-type:access_token&
requested_token_type=urn:ietf:params:oauth:token-type:access_token&
scope=https://www.googleapis.com/auth/cloud-platform
```

**What GCP Does:**
1. Validates the federated token
2. Extracts the GCP principal
3. Checks IAM policy: Does this principal have `iam.workloadIdentityUser` on the service account?
4. If yes, issues an access token with the service account's identity
5. Access token inherits all IAM permissions of the service account

**Response:**
```json
{
  "access_token": "ya29.gcp-access-token-here...",
  "issued_token_type": "urn:ietf:params:oauth:token-type:access_token",
  "token_type": "Bearer",
  "expires_in": 3600
}
```

**Access Token Characteristics:**
- Short-lived (typically 1 hour)
- Acts as the service account
- Can access any GCP resource the SA has permissions for
- Standard OAuth 2.0 Bearer token

## Attribute Mapping

Attribute mapping transforms JWT claims into GCP principal attributes.

### Mapping Syntax

```bash
--attribute-mapping="google.subject=assertion.sub,attribute.email=assertion.email"
```

**Format:** `gcp_attribute=jwt_claim`

### Reserved Mappings

1. **google.subject** (required)
   - Becomes the principal identifier
   - Example: `assertion.sub` → principal is `external-user-123`

2. **google.groups** (optional)
   - Maps to groups
   - Example: `assertion.groups` → principal has group memberships

### Custom Attribute Mappings

Prefix with `attribute.`:
```bash
attribute.email=assertion.email
attribute.department=assertion.dept
attribute.environment=assertion.env
```

These can be used in:
- Attribute conditions
- IAM policy bindings
- Logging/auditing

### Principal Identifiers

After mapping, the principal becomes:
```
principal://iam.googleapis.com/projects/PROJECT_NUM/locations/global/workloadIdentityPools/POOL_ID/subject/SUBJECT_VALUE
```

Example:
```
principal://iam.googleapis.com/projects/123456789/locations/global/workloadIdentityPools/external-identity-pool/subject/external-user-123
```

## Attribute Conditions

Add CEL (Common Expression Language) conditions to restrict access:

```bash
--attribute-condition="assertion.environment == 'production' && assertion.email.endsWith('@example.com')"
```

### Common Patterns

**Check email domain:**
```cel
assertion.email.endsWith('@company.com')
```

**Check multiple audiences:**
```cel
assertion.aud in ['aud1', 'aud2', 'aud3']
```

**Check groups:**
```cel
'admin-group' in assertion.groups
```

**Complex condition:**
```cel
assertion.environment == 'production' &&
assertion.email.endsWith('@company.com') &&
assertion.exp > now()
```

## JWKS (Production Setup)

In production, you need to host a JWKS (JSON Web Key Set) endpoint.

### Convert PEM to JWK

```bash
# Extract public key components
openssl rsa -pubin -in public_key.pem -text -noout
```

### JWKS Format

```json
{
  "keys": [
    {
      "kty": "RSA",
      "use": "sig",
      "kid": "key-id-1",
      "alg": "RS256",
      "n": "<base64url-encoded-modulus>",
      "e": "AQAB"
    }
  ]
}
```

**Fields:**
- `kty`: Key type (RSA)
- `use`: Usage (sig = signature)
- `kid`: Key ID (for rotation)
- `alg`: Algorithm (RS256)
- `n`: Modulus (public key component)
- `e`: Exponent (typically 65537 = AQAB in base64)

### Hosting JWKS

Host at a public HTTPS endpoint:
```
https://my-idp.example.com/.well-known/jwks.json
```

Configure provider:
```bash
gcloud iam workload-identity-pools providers create-oidc external-jwt-provider \
  ...
  --jwk-json-path="https://my-idp.example.com/.well-known/jwks.json"
```

GCP will:
1. Fetch JWKS periodically
2. Cache the keys
3. Use them to verify JWT signatures
4. Support key rotation via `kid`

## Security Model

### Trust Boundaries

1. **External IdP → GCP**
   - Trust established via JWKS (public key verification)
   - GCP validates: signature, issuer, audience, expiration
   - Attribute conditions provide additional validation

2. **Federated Token → Service Account**
   - Trust established via IAM policy
   - `workloadIdentityUser` role grants impersonation
   - Can restrict by specific principal or use wildcards

3. **Access Token → GCP Resources**
   - Standard GCP IAM
   - Service account's permissions apply
   - Same as any other service account authentication

### Why Two Exchanges?

The two-step exchange provides security separation:

1. **First exchange (JWT → Federated Token)**
   - Validates external identity
   - Creates GCP-internal representation
   - Doesn't grant resource access yet

2. **Second exchange (Federated → Access)**
   - Validates authorization (IAM policy)
   - Grants actual resource access
   - Audit trail shows which external identity accessed what

This separation allows:
- Different policies for identity validation vs. authorization
- Auditing of external identities in GCP logs
- Fine-grained control over which external identities can use which service accounts

## OAuth 2.0 Token Exchange (RFC 8693)

Workload Identity Federation uses the OAuth 2.0 Token Exchange standard.

### Grant Type
```
urn:ietf:params:oauth:grant-type:token-exchange
```

### Token Types

**JWT:**
```
urn:ietf:params:oauth:token-type:jwt
```

**Access Token:**
```
urn:ietf:params:oauth:token-type:access_token
```

**Refresh Token:**
```
urn:ietf:params:oauth:token-type:refresh_token
```

### Parameters

- `grant_type`: Always token-exchange
- `audience`: Who the new token is for
- `subject_token`: The token you're presenting
- `subject_token_type`: Type of token you're presenting
- `requested_token_type`: Type of token you want back
- `scope`: Requested permissions

## GCP-Specific Extensions

### Audience Format

For workload identity pool:
```
//iam.googleapis.com/projects/PROJECT_NUM/locations/global/workloadIdentityPools/POOL_ID/providers/PROVIDER_ID
```

For service account:
```
//iam.googleapis.com/projects/PROJECT_NUM/serviceAccounts/SA_EMAIL
```

### Scopes

Most common:
```
https://www.googleapis.com/auth/cloud-platform
```

API-specific:
```
https://www.googleapis.com/auth/pubsub
https://www.googleapis.com/auth/compute
```

## Debugging

### Decode JWT

Online: https://jwt.io

Command line:
```bash
echo "eyJhbGc..." | cut -d. -f2 | base64 -d 2>/dev/null | jq
```

### Inspect Access Token

```bash
curl -H "Authorization: Bearer $(cat gcp_access_token.txt)" \
  https://www.googleapis.com/oauth2/v1/tokeninfo
```

### Test Token Exchange Manually

```bash
# Step 1: JWT → Federated
curl -X POST https://sts.googleapis.com/v1/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=urn:ietf:params:oauth:grant-type:token-exchange" \
  -d "audience=//iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/pool/providers/provider" \
  -d "subject_token=$(cat external_token.jwt)" \
  -d "subject_token_type=urn:ietf:params:oauth:token-type:jwt" \
  -d "requested_token_type=urn:ietf:params:oauth:token-type:access_token" \
  -d "scope=https://www.googleapis.com/auth/cloud-platform"

# Step 2: Federated → Access
curl -X POST https://sts.googleapis.com/v1/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=urn:ietf:params:oauth:grant-type:token-exchange" \
  -d "audience=//iam.googleapis.com/projects/123/serviceAccounts/sa@project.iam.gserviceaccount.com" \
  -d "subject_token=FEDERATED_TOKEN" \
  -d "subject_token_type=urn:ietf:params:oauth:token-type:access_token" \
  -d "requested_token_type=urn:ietf:params:oauth:token-type:access_token" \
  -d "scope=https://www.googleapis.com/auth/cloud-platform"
```

## Common Errors

### "Invalid JWT signature"
- JWKS not configured or unreachable
- Wrong public key
- JWT signed with different key than in JWKS

### "Invalid issuer"
- JWT `iss` doesn't match provider's `--issuer-uri`

### "Invalid audience"
- JWT `aud` not in provider's `--allowed-audiences`

### "Attribute condition evaluation failed"
- JWT missing required claims
- Condition logic doesn't match JWT claims

### "Permission denied" (Step 2)
- Service account doesn't have `workloadIdentityUser` binding
- Principal identifier doesn't match IAM policy
- Check: `gcloud iam service-accounts get-iam-policy SA_EMAIL`

## Performance Considerations

### Token Caching

Both federated and access tokens are valid for ~1 hour. Cache them:

```go
type TokenCache struct {
    token      string
    expiresAt  time.Time
}

func (c *TokenCache) GetToken() (string, error) {
    if time.Now().Before(c.expiresAt.Add(-5 * time.Minute)) {
        return c.token, nil
    }
    // Refresh token
    newToken, expiresIn := exchangeToken()
    c.token = newToken
    c.expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
    return c.token, nil
}
```

### Parallel Requests

If making multiple API calls, reuse the same access token:

```go
token := getAccessToken() // Once
for _, resource := range resources {
    go callAPI(resource, token) // Many times
}
```

## References

- [RFC 7519 - JSON Web Token](https://datatracker.ietf.org/doc/html/rfc7519)
- [RFC 8693 - OAuth 2.0 Token Exchange](https://datatracker.ietf.org/doc/html/rfc8693)
- [RFC 7517 - JSON Web Key (JWK)](https://datatracker.ietf.org/doc/html/rfc7517)
- [GCP Workload Identity Federation](https://cloud.google.com/iam/docs/workload-identity-federation)
- [GCP STS API](https://cloud.google.com/iam/docs/reference/sts/rest)
- [Common Expression Language (CEL)](https://github.com/google/cel-spec)
