# Required GitHub Secrets

This document lists all the secrets that need to be configured in your GitHub repository for the build workflow to work.

## macOS Code Signing & Notarization

These secrets are required for signing and notarizing the macOS app:

### `NOTARY_KEY_ID`
- **Description**: Your Apple Notary Service API Key ID
- **How to get it**: 
  1. Go to https://appstoreconnect.apple.com/access/api
  2. Create a new API key
  3. Copy the Key ID
- **Example**: `ABC123DEF4`

### `NOTARY_ISSUER_ID`
- **Description**: Your Apple Notary Service Issuer ID
- **How to get it**: 
  1. Go to https://appstoreconnect.apple.com/access/api
  2. The Issuer ID is shown at the top of the page
- **Example**: `12345678-1234-1234-1234-123456789012`

### `NOTARY_P8_KEY`
- **Description**: The contents of your `.p8` private key file
- **How to get it**: 
  1. Download the `.p8` file when creating the API key
  2. Copy the entire contents of the file (including `-----BEGIN PRIVATE KEY-----` and `-----END PRIVATE KEY-----`)
  3. Paste it as a secret (multiline)
- **Example**: 
  ```
  -----BEGIN PRIVATE KEY-----
  MIGTAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBHkwdwIBAQQg...
  -----END PRIVATE KEY-----
  ```

## How to Add Secrets

1. Go to your GitHub repository
2. Click **Settings** → **Secrets and variables** → **Actions**
3. Click **New repository secret**
4. Enter the name and value
5. Click **Add secret**

## Notes

- The `NOTARY_P8_PATH` environment variable in `wails.json` is not used in the GitHub Actions workflow
- The workflow creates a temporary `notary_key.p8` file from the `NOTARY_P8_KEY` secret
- All secrets are automatically masked in workflow logs

