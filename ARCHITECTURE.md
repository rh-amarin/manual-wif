# Architecture and Flow Diagrams

Visual representation of how Workload Identity Federation works.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Your Environment                            │
│  ┌──────────────────┐                                              │
│  │ External System  │                                              │
│  │ (AWS, On-prem,   │                                              │
│  │  GitHub, etc.)   │                                              │
│  └────────┬─────────┘                                              │
│           │                                                         │
│           │ 1. Creates identity proof                             │
│           │    (JWT token)                                         │
│           │                                                         │
│           ▼                                                         │
│  ┌──────────────────┐                                              │
│  │  JWT Generator   │                                              │
│  │  - Signs with    │                                              │
│  │    private key   │                                              │
│  │  - Adds claims   │                                              │
│  └────────┬─────────┘                                              │
│           │                                                         │
└───────────┼─────────────────────────────────────────────────────────┘
            │
            │ JWT Token
            │ (External Identity Proof)
            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        Google Cloud Platform                        │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────┐      │
│  │                     STS API                               │      │
│  │  ┌─────────────────────────────────────────────────┐     │      │
│  │  │  Step 1: Validate External Identity             │     │      │
│  │  │  - Check JWT signature (via JWKS)               │     │      │
│  │  │  - Validate issuer, audience, expiration        │     │      │
│  │  │  - Apply attribute mappings                     │     │      │
│  │  │  - Evaluate attribute conditions                │     │      │
│  │  └─────────────────┬───────────────────────────────┘     │      │
│  │                    │                                     │      │
│  │                    ▼                                     │      │
│  │         Returns Federated Token                         │      │
│  │                    │                                     │      │
│  │                    ▼                                     │      │
│  │  ┌─────────────────────────────────────────────────┐     │      │
│  │  │  Step 2: Service Account Impersonation          │     │      │
│  │  │  - Validate federated token                     │     │      │
│  │  │  - Check IAM policy (workloadIdentityUser)      │     │      │
│  │  │  - Issue access token as service account        │     │      │
│  │  └─────────────────┬───────────────────────────────┘     │      │
│  └────────────────────┼───────────────────────────────────┘      │
│                       │                                           │
│                       ▼                                           │
│            Returns Access Token                                   │
│                       │                                           │
│                       │                                           │
│  ┌────────────────────┼───────────────────────────────────┐      │
│  │                    │     IAM & Resources               │      │
│  │                    ▼                                   │      │
│  │       ┌──────────────────────┐                         │      │
│  │       │  Service Account     │                         │      │
│  │       │  wif-sa@project.iam  │                         │      │
│  │       └──────────┬───────────┘                         │      │
│  │                  │                                     │      │
│  │                  │ Has permissions                     │      │
│  │                  ▼                                     │      │
│  │       ┌──────────────────────┐                         │      │
│  │       │   GCP Resources      │                         │      │
│  │       │   - Pub/Sub          │                         │      │
│  │       │   - Cloud Storage    │                         │      │
│  │       │   - Compute Engine   │                         │      │
│  │       │   - etc.             │                         │      │
│  │       └──────────────────────┘                         │      │
│  └────────────────────────────────────────────────────────┘      │
└─────────────────────────────────────────────────────────────────────┘
```

## Detailed Token Flow

```
┌──────────────┐
│ External App │
└──────┬───────┘
       │
       │ Creates JWT
       │ {
       │   "iss": "https://external-idp.example.com",
       │   "sub": "user-123",
       │   "aud": "gcp-workload-identity",
       │   "exp": 1234567890
       │ }
       │
       ▼
┌────────────────────────────────────────────────┐
│ Step 1: JWT → Federated Token                 │
├────────────────────────────────────────────────┤
│                                                │
│  POST https://sts.googleapis.com/v1/token      │
│  {                                             │
│    "grant_type": "token-exchange",             │
│    "subject_token": "<JWT>",                   │
│    "subject_token_type": "jwt",                │
│    "audience": "//iam.googleapis.com/          │
│      projects/123/locations/global/            │
│      workloadIdentityPools/pool/               │
│      providers/provider"                       │
│  }                                             │
│                                                │
│  ┌──────────────────────────────────────┐     │
│  │ GCP Validates:                       │     │
│  │ ✓ Signature (using JWKS)             │     │
│  │ ✓ Issuer matches config              │     │
│  │ ✓ Audience matches config            │     │
│  │ ✓ Not expired                        │     │
│  │ ✓ Attribute conditions pass          │     │
│  └──────────────────────────────────────┘     │
│                                                │
│  Returns:                                      │
│  {                                             │
│    "access_token": "ya29.fed...",              │
│    "token_type": "Bearer",                     │
│    "expires_in": 3600                          │
│  }                                             │
│                                                │
└────────────────┬───────────────────────────────┘
                 │ Federated Token
                 │ (GCP-internal representation)
                 │
                 ▼
┌────────────────────────────────────────────────┐
│ Step 2: Federated Token → Access Token        │
├────────────────────────────────────────────────┤
│                                                │
│  POST https://sts.googleapis.com/v1/token      │
│  {                                             │
│    "grant_type": "token-exchange",             │
│    "subject_token": "<federated_token>",       │
│    "subject_token_type": "access_token",       │
│    "audience": "//iam.googleapis.com/          │
│      projects/123/serviceAccounts/             │
│      sa@project.iam.gserviceaccount.com"       │
│  }                                             │
│                                                │
│  ┌──────────────────────────────────────┐     │
│  │ GCP Checks IAM Policy:               │     │
│  │                                      │     │
│  │ Does principal have role             │     │
│  │ "roles/iam.workloadIdentityUser"     │     │
│  │ on the service account?              │     │
│  │                                      │     │
│  │ principal://iam.googleapis.com/      │     │
│  │   projects/123/.../pool/             │     │
│  │   subject/user-123                   │     │
│  │                                      │     │
│  │ If YES → issue access token          │     │
│  └──────────────────────────────────────┘     │
│                                                │
│  Returns:                                      │
│  {                                             │
│    "access_token": "ya29.gcp...",              │
│    "token_type": "Bearer",                     │
│    "expires_in": 3600                          │
│  }                                             │
│                                                │
└────────────────┬───────────────────────────────┘
                 │ Access Token
                 │ (Acts as Service Account)
                 │
                 ▼
┌────────────────────────────────────────────────┐
│ Step 3: Call GCP API                          │
├────────────────────────────────────────────────┤
│                                                │
│  GET https://pubsub.googleapis.com/v1/         │
│      projects/my-project/topics                │
│  Headers:                                      │
│    Authorization: Bearer ya29.gcp...           │
│                                                │
│  ┌──────────────────────────────────────┐     │
│  │ GCP Validates:                       │     │
│  │ ✓ Token is valid and not expired     │     │
│  │ ✓ Token represents sa@project.iam    │     │
│  │ ✓ SA has roles/pubsub.viewer         │     │
│  │ ✓ Returns list of topics             │     │
│  └──────────────────────────────────────┘     │
│                                                │
│  Returns:                                      │
│  {                                             │
│    "topics": [                                 │
│      { "name": "projects/.../topics/topic1" }, │
│      { "name": "projects/.../topics/topic2" }  │
│    ]                                           │
│  }                                             │
│                                                │
└────────────────────────────────────────────────┘
```

## Component Relationships

```
┌─────────────────────────────────────────────────────────────┐
│                    Workload Identity Pool                   │
│  Namespace: external-identity-pool                          │
│                                                             │
│  ┌───────────────────────────────────────────────────┐     │
│  │         Workload Identity Provider                │     │
│  │  Config: external-jwt-provider                    │     │
│  │                                                   │     │
│  │  Settings:                                        │     │
│  │  • Issuer URI: https://external-idp.example.com   │     │
│  │  • Allowed Audiences: gcp-workload-identity       │     │
│  │  • JWKS URI: https://external-idp/.../jwks.json   │     │
│  │  • Attribute Mapping:                             │     │
│  │    - google.subject = assertion.sub               │     │
│  │    - attribute.email = assertion.email            │     │
│  │  • Attribute Condition:                           │     │
│  │    - assertion.aud == 'gcp-workload-identity'     │     │
│  │                                                   │     │
│  └───────────────────┬───────────────────────────────┘     │
│                      │                                     │
│                      │ Maps to GCP Principal:              │
│                      │                                     │
│                      ▼                                     │
│  principal://iam.googleapis.com/projects/123/              │
│    locations/global/workloadIdentityPools/                 │
│    external-identity-pool/subject/user-123                 │
│                      │                                     │
└──────────────────────┼─────────────────────────────────────┘
                       │
                       │ IAM Binding:
                       │ roles/iam.workloadIdentityUser
                       │
                       ▼
         ┌──────────────────────────────┐
         │      Service Account         │
         │  wif-sa@project.iam          │
         │                              │
         │  IAM Roles:                  │
         │  • roles/pubsub.viewer       │
         │  • roles/storage.objectViewer│
         │  • ...                       │
         └──────────────────────────────┘
```

## Trust Chain

```
1. Your Private Key
   │
   │ Signs
   ▼
2. JWT Token
   │
   │ Verified by
   ▼
3. GCP (using JWKS/Public Key)
   │
   │ Creates
   ▼
4. GCP Principal
   │
   │ IAM Policy Check
   ▼
5. Service Account
   │
   │ IAM Permissions
   ▼
6. GCP Resources
```

## Security Boundaries

```
┌─────────────────────────────────────────┐
│  Boundary 1: Identity Verification       │
├─────────────────────────────────────────┤
│  • Validates JWT signature              │
│  • Checks issuer and audience           │
│  • Enforces attribute conditions        │
│  • Creates GCP principal                │
│                                         │
│  Question: "Who are you?"               │
│  Answer: "I'm user-123 from external"   │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│  Boundary 2: Authorization              │
├─────────────────────────────────────────┤
│  • Checks IAM policy                    │
│  • Verifies workloadIdentityUser role   │
│  • Grants service account token         │
│                                         │
│  Question: "What can you do?"           │
│  Answer: "Impersonate service account"  │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│  Boundary 3: Resource Access            │
├─────────────────────────────────────────┤
│  • Standard GCP IAM                     │
│  • Service account permissions apply    │
│  • Resource-level access control        │
│                                         │
│  Question: "Which resources?"           │
│  Answer: "Whatever the SA can access"   │
└─────────────────────────────────────────┘
```

## Token Lifecycle

```
Time: 0min                                     60min
  │                                              │
  │ Create JWT                                   │
  ├──────────┐                                   │
  │          │ Valid for 1 hour                  │
  │          │                                   │
  │          ├────────────────────────────────────┤ Expires
  │          │
  │ Exchange for Federated Token
  ├──────────┐
  │          │ Valid for ~1 hour
  │          │
  │          ├─────────────────────────────────────┤ Expires
  │          │
  │ Exchange for Access Token
  ├──────────┐
  │          │ Valid for ~1 hour
  │          │
  │          │ Use for API calls ──────────┐
  │          │                             │
  │          │                             ▼
  │          │                    ┌──────────────┐
  │          │                    │ Pub/Sub API  │
  │          │                    │ Storage API  │
  │          │                    │ Compute API  │
  │          │                    │ etc.         │
  │          │                    └──────────────┘
  │          │
  │          ├─────────────────────────────────────┤ Expires
  │
  │ Refresh flow (create new JWT and repeat)
  ▼
```

## Comparison: Traditional vs WIF

### Traditional (Service Account Key)

```
┌──────────────┐
│ External App │
└──────┬───────┘
       │
       │ Downloads once
       │
       ▼
┌─────────────────────┐
│ SA Key JSON File    │
│ {                   │
│   "private_key": ...,│
│   "client_email": ..│
│ }                   │
└──────┬──────────────┘
       │
       │ Stored on disk (security risk!)
       │ Valid forever until revoked
       │
       ▼
┌─────────────────────┐
│ Sign JWT with       │
│ private key         │
└──────┬──────────────┘
       │
       ▼
┌─────────────────────┐
│ Exchange for        │
│ access token        │
└──────┬──────────────┘
       │
       ▼
┌─────────────────────┐
│ Access GCP          │
└─────────────────────┘

Risks:
• Key can leak
• Key lives forever
• Hard to rotate
• No external identity context
```

### Workload Identity Federation

```
┌──────────────┐
│ External App │
└──────┬───────┘
       │
       │ No GCP credentials!
       │
       ▼
┌─────────────────────┐
│ Create JWT with     │
│ own private key     │
└──────┬──────────────┘
       │
       │ Short-lived, contextual
       │
       ▼
┌─────────────────────┐
│ Exchange via WIF    │
│ (two steps)         │
└──────┬──────────────┘
       │
       ▼
┌─────────────────────┐
│ Access GCP          │
└─────────────────────┘

Benefits:
• No GCP credentials to leak
• Tokens are short-lived
• External identity preserved
• Automatic rotation
• Auditable (shows external identity)
```

## Multi-Cloud Example

```
┌──────────────────────────────────────────────┐
│              AWS Account                      │
│                                              │
│  ┌────────────────┐                          │
│  │ EC2 Instance   │                          │
│  │                │                          │
│  │ Gets AWS       │                          │
│  │ credentials    │                          │
│  │ from IMDS      │                          │
│  └────────┬───────┘                          │
│           │                                  │
└───────────┼──────────────────────────────────┘
            │
            │ AWS credentials
            │ (GetCallerIdentity token)
            ▼
┌──────────────────────────────────────────────┐
│         Google Cloud Platform                │
│                                              │
│  Workload Identity Provider configured       │
│  to accept AWS tokens:                       │
│  • Issuer: AWS STS                           │
│  • Validates AWS signatures                  │
│  • Maps AWS ARN to GCP principal             │
│                                              │
│  principal://iam.googleapis.com/.../         │
│    subject/arn:aws:sts::123:assumed-role/... │
│           │                                  │
│           ▼                                  │
│  Service Account                             │
│  → Access GCP Resources                      │
└──────────────────────────────────────────────┘

Result: AWS workload can access GCP resources
        without storing GCP credentials!
```

## Key Takeaways

1. **No Long-Lived Credentials**: External systems never receive GCP service account keys

2. **Short-Lived Tokens**: All tokens expire in ~1 hour, limiting exposure

3. **Identity Context**: GCP knows the external identity (e.g., AWS ARN, GitHub repo)

4. **Two-Step Security**: Separate validation (who) from authorization (what)

5. **Standard Protocols**: Uses OAuth 2.0 and JWT standards

6. **Flexible Mapping**: Attribute mappings adapt external claims to GCP model

7. **Defense in Depth**: Multiple validation points (signature, issuer, audience, conditions, IAM)
