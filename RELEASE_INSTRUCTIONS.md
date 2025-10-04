# Quick Release Instructions for stashr v0.0.1

This is a simplified guide to publish stashr to Homebrew. Follow these steps in order.

## Prerequisites Checklist

- [ ] You have a GitHub account
- [ ] Your `stashr` repository exists at `github.com/harshalranjhani/stashr`
- [ ] Your tap repository exists at `github.com/harshalranjhani/homebrew-harshalranjhani`
- [ ] Both repositories are public
- [ ] You have push access to both repositories

## Step-by-Step Instructions

### 1. Prepare Your Main Repository

```bash
# Navigate to the stashr directory
cd /Users/harshalranjhani/Developer/stashr

# (Optional) Rename directory to match project name
cd ..
mv stashr stashr
cd stashr

# Initialize git and push (if not already done)
git init
git add .
git commit -m "Initial release v0.0.1"
git branch -M main
git remote add origin https://github.com/harshalranjhani/stashr.git
git push -u origin main
```

### 2. Create GitHub Release (Automated)

```bash
# Create and push tag - This triggers GitHub Actions to build binaries
git tag -a v0.0.1 -m "Release v0.0.1"
git push origin v0.0.1

# Wait 2-3 minutes for GitHub Actions to complete
# Check status at: https://github.com/harshalranjhani/stashr/actions
```

**OR** use the automated script:

```bash
./scripts/setup-homebrew.sh
```

### 3. Get SHA256 Hash

After the GitHub release is created (wait ~2 minutes), get the SHA256:

```bash
# Download and calculate SHA256 in one command
curl -L https://github.com/harshalranjhani/stashr/archive/refs/tags/v0.0.1.tar.gz | shasum -a 256
```

Copy the hash (first part before the dash).

### 4. Update Homebrew Formula

```bash
# Clone your tap repository
cd ~/Developer
git clone https://github.com/harshalranjhani/homebrew-harshalranjhani.git
cd homebrew-harshalranjhani

# Create Formula directory if it doesn't exist
mkdir -p Formula

# Copy the formula template
cp /Users/harshalranjhani/Developer/stashr/Formula/stashr.rb ./Formula/

# Edit Formula/stashr.rb and replace the empty sha256 "" with the hash from step 3
# Example: sha256 "abc123def456..."

# Commit and push
git add Formula/stashr.rb
git commit -m "Add stashr formula v0.0.1"
git push origin main
```

### 5. Test Installation

```bash
# Test the formula locally first
brew install --build-from-source ~/Developer/homebrew-harshalranjhani/Formula/stashr.rb

# If it works, uninstall and try from tap
brew uninstall stashr

# Install from tap
brew tap harshalranjhani/harshalranjhani
brew install stashr

# Verify
stashr --version
# Should show: stashr version 0.0.1
```

### 6. Share with Users

Tell users to install with:

```bash
brew tap harshalranjhani/harshalranjhani
brew install stashr
```

## Future Updates

When you want to release v0.0.2, v0.1.0, etc.:

1. Update version in `internal/version/version.go`
2. Commit changes
3. Create and push new tag: `git tag -a v0.0.2 -m "Release v0.0.2" && git push origin v0.0.2`
4. Get new SHA256: `curl -L https://github.com/harshalranjhani/stashr/archive/refs/tags/v0.0.2.tar.gz | shasum -a 256`
5. Update `Formula/stashr.rb` in tap repo with new version and SHA256
6. Push to tap repo

## Troubleshooting

### GitHub Actions failed
- Check the Actions tab: https://github.com/harshalranjhani/stashr/actions
- Make sure `.github/workflows/release.yml` exists and is correct

### SHA256 download fails
- Wait a few minutes - GitHub might still be processing the release
- Check if the release exists: https://github.com/harshalranjhani/stashr/releases

### Homebrew formula errors
```bash
# Audit the formula
brew audit --strict ~/Developer/homebrew-harshalranjhani/Formula/stashr.rb

# Common issues:
# - Wrong SHA256: Recalculate and update
# - Wrong URL: Check that tag exists on GitHub
# - Build fails: Test with: brew install --build-from-source --verbose ./Formula/stashr.rb
```

### Users can't find the tap
- Verify repo name is exactly: `homebrew-harshalranjhani` (starts with `homebrew-`)
- Verify repo is public
- Verify `Formula/` directory exists with `stashr.rb` inside

## Files Created for You

✅ **Version management**:
- `internal/version/version.go` - Version constants

✅ **Homebrew**:
- `Formula/stashr.rb` - Homebrew formula template
- `HOMEBREW_PUBLISHING.md` - Detailed publishing guide
- `scripts/setup-homebrew.sh` - Automated setup script

✅ **CI/CD**:
- `.github/workflows/release.yml` - Automated builds on tag push

## Quick Commands Reference

```bash
# Build locally
go build -o stashr

# Test version
./stashr --version

# Create release
git tag -a v0.0.1 -m "Release v0.0.1" && git push origin v0.0.1

# Get SHA256
curl -L https://github.com/harshalranjhani/stashr/archive/refs/tags/v0.0.1.tar.gz | shasum -a 256

# Update tap (after editing SHA256 in Formula/stashr.rb)
cd ~/Developer/homebrew-harshalranjhani
git add Formula/stashr.rb
git commit -m "Add stashr v0.0.1"
git push

# Install from tap
brew tap harshalranjhani/harshalranjhani
brew install stashr
```

## Need Help?

- Detailed guide: See `HOMEBREW_PUBLISHING.md`
- Automated script: Run `./scripts/setup-homebrew.sh`
- GitHub Actions logs: https://github.com/harshalranjhani/stashr/actions
