# JWK Upload Guide for GCP Workload Identity Federation

## What You Have

After running `make step1` (or `go run step1_generate_keys.go && go run step1b_generate_jwk.go`), you now have:

1. **private_key.pem** - Private key for signing JWTs (keep secret!)
2. **public_key.pem** - Public key in PEM format
3. **public_key.jwk** - Single JSON Web Key
4. **public_key.jwks** - JSON Web Key Set (this is what GCP needs)

## Why You Need JWK

GCP Workload Identity Federation needs the public key in JWK format to verify the signature of your JWT tokens. Without this, GCP cannot validate that the tokens are authentic.

## Two Options for Using JWK with GCP

### Option 1: Host JWKS at a Public URL (Recommended for Production)

1. **Host the JWKS file** at a publicly accessible HTTPS URL:
   ```bash
   # Example using GitHub Pages, S3, or any web server
   # The URL might be something like:
   # https://yourdomain.com/.well-known/jwks.json
   ```

2. **Configure the Workload Identity Provider** with the JWKS URL:
   ```bash
   gcloud iam workload-identity-pools providers create-oidc external-jwt-provider \
     --location=global \
     --workload-identity-pool=external-identity-pool \
     --issuer-uri="https://47c2455ce3d3.ngrok-free.app/" \
     --allowed-audiences="gcp-workload-identity" \
     --attribute-mapping="google.subject=assertion.sub" \
     --jwk-json-path="https://yourdomain.com/.well-known/jwks.json"
   ```

### Option 2: Use Inline JWK (For Testing/Development)

You can provide the JWK content directly in the gcloud command:

```bash
# Read the JWKS file content
JWKS_CONTENT=$(cat public_key.jwks)

# Create provider with inline JWK
gcloud iam workload-identity-pools providers create-oidc external-jwt-provider \
  --location=global \
  --workload-identity-pool=external-identity-pool \
  --issuer-uri="https://47c2455ce3d3.ngrok-free.app/" \
  --allowed-audiences="gcp-workload-identity" \
  --attribute-mapping="google.subject=assertion.sub" \
  --jwk-json-path=<(echo "$JWKS_CONTENT")
```

### Option 3: Update Existing Provider

If you already created a provider without JWKS, update it:

```bash
gcloud iam workload-identity-pools providers update-oidc external-jwt-provider \
  --location=global \
  --workload-identity-pool=external-identity-pool \
  --jwk-json-path="https://yourdomain.com/.well-known/jwks.json"
```

## Quick Test with ngrok (For Local Development)

If you want to test locally:

1. **Install ngrok** (if not already installed)

2. **Serve the JWKS file**:
   ```bash
   # Simple Python server
   python3 -m http.server 8080
   ```

3. **Expose with ngrok**:
   ```bash
   ngrok http 8080
   ```

4. **Use the ngrok URL**:
   ```bash
   # If ngrok gives you https://abc123.ngrok-free.app
   # Your JWKS URL would be:
   # https://abc123.ngrok-free.app/public_key.jwks

   gcloud iam workload-identity-pools providers create-oidc external-jwt-provider \
     --location=global \
     --workload-identity-pool=external-identity-pool \
     --issuer-uri="https://47c2455ce3d3.ngrok-free.app/" \
     --allowed-audiences="gcp-workload-identity" \
     --attribute-mapping="google.subject=assertion.sub" \
     --jwk-json-path="https://abc123.ngrok-free.app/public_key.jwks"
   ```

## Verify It Works

After configuring the provider with JWK:

1. **Check the provider configuration**:
   ```bash
   gcloud iam workload-identity-pools providers describe external-jwt-provider \
     --location=global \
     --workload-identity-pool=external-identity-pool
   ```

2. **Test token exchange**:
   ```bash
   make step2  # Create JWT
   make step3  # Exchange for GCP token
   ```

3. **If successful**, you should see:
   - JWT signature is verified by GCP
   - Access token is returned
   - You can use the token to access GCP resources

## Troubleshooting

### Error: "Failed to verify JWT signature"
- Ensure the JWKS URL is publicly accessible
- Check that the `kid` in the JWT header matches the `kid` in the JWK
- Verify the JWT was signed with the private key matching the public key in the JWK

### Error: "Unable to fetch JWKS"
- Ensure the URL is HTTPS (not HTTP)
- Check that the URL is publicly accessible (no authentication required)
- Verify the JWKS file contains valid JSON

### Error: "Invalid JWK format"
- Ensure you're uploading the JWKS file (with "keys" array), not just the JWK
- Validate the JSON format is correct

## What's in the JWKS File

```json
{
  "keys": [
    {
      "kty": "RSA",           // Key type
      "use": "sig",           // Use for signatures
      "kid": "key-1",         // Key ID (matches JWT header)
      "alg": "RS256",         // Algorithm
      "n": "...",             // RSA modulus (base64url)
      "e": "AQAB"             // RSA exponent (base64url)
    }
  ]
}
```

## Security Notes

- **Never commit** the private key (private_key.pem) to version control
- The public key (JWKS) is safe to host publicly
- In production, implement key rotation
- Use HTTPS for the JWKS endpoint
- Consider using a CDN for high availability

## Next Steps

After uploading the JWK to GCP:
1. Run `make step2` to create a JWT with the correct `kid` header
2. Run `make step3` to exchange the JWT for a GCP access token
3. Run `make step4` to test accessing GCP resources
