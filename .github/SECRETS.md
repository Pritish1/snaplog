# Required GitHub Secrets

This document lists all the secrets that need to be configured in your GitHub repository for the build workflow to work.

## macOS Code Signing & Notarization

These secrets are required for signing and notarizing the macOS app:

### `DEVELOPER_ID_CERT`
- **Description**: Your Developer ID certificate in base64-encoded `.p12` format
- **How to get it**: 
  1. Export your Developer ID certificate from Keychain Access on macOS
  2. Select the certificate → Right-click → Export
  3. Choose `.p12` format and set a password
  4. Convert to base64: `base64 -i certificate.p12 | pbcopy` (macOS) or `certutil -encode certificate.p12 certificate.txt` (Windows)
  5. Copy the base64 content (without BEGIN/END markers if using certutil)
- **Example**: Base64-encoded string of your .p12 file

### `DEVELOPER_ID_CERT_PASSWORD`
- **Description**: The password you set when exporting the `.p12` certificate
- **How to get it**: The password you entered when exporting the certificate from Keychain Access
- **Example**: `your-certificate-password`

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

