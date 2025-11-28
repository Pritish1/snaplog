# Build and Release Process

## Overview

This project uses GitHub Actions to automatically build and release the app for Windows and macOS.

## Version Management

- Version is stored in the `VERSION` file (currently `0.1.0`)
- Releases are triggered by creating a git tag: `v1.0.0`, `v1.0.1`, etc.
- The workflow extracts the version from the tag name

## Workflow Triggers

The workflow runs automatically when:
1. **Tag push**: When you push a tag starting with `v` (e.g., `v1.0.0`)
2. **Manual trigger**: You can manually trigger it from the Actions tab

## Build Process

### Windows
- Builds on `windows-latest` runner
- No code signing
- Output: `snaplog.exe`

### macOS
- Builds on `macos-latest` runner
- Code signs with Developer ID
- Notarizes with Apple Notary Service
- Creates DMG for distribution
- Output: `snaplog.app` and `snaplog.dmg`

## Creating a Release

1. **Update version** (optional - can use git tag directly):
   ```bash
   echo "1.0.0" > VERSION
   git add VERSION
   git commit -m "Bump version to 1.0.0"
   ```

2. **Create and push a tag**:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

3. **Workflow runs automatically**:
   - Builds Windows and macOS apps
   - Creates a GitHub Release
   - Uploads artifacts to the release

## Required Secrets

Before the first build, you must configure these secrets in GitHub:
- `NOTARY_KEY_ID` - Apple Notary Service Key ID
- `NOTARY_ISSUER_ID` - Apple Notary Service Issuer ID  
- `NOTARY_P8_KEY` - Contents of your `.p8` private key file

See [SECRETS.md](./SECRETS.md) for detailed instructions.

## Manual Testing

To test the workflow without creating a release:

1. Go to **Actions** tab in GitHub
2. Click **Build and Release** workflow
3. Click **Run workflow**
4. Select branch and click **Run workflow**

This will build the apps but won't create a release (only tags trigger releases).

## Troubleshooting

### macOS Build Fails
- Check that all secrets are set correctly
- Verify the Developer ID matches your certificate
- Ensure the entitlements file exists at `assets/darwin/entitlements.plist`

### Notarization Fails
- Verify the `.p8` key file contents are correct (include BEGIN/END lines)
- Check that the Key ID and Issuer ID match your Apple Developer account
- Ensure the app is properly code signed before notarization

### Windows Build Fails
- Check that Wails CLI is installed correctly
- Verify Node.js and npm are available
- Check frontend build output for errors

