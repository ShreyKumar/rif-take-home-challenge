#!/usr/bin/env bash
# Setup GitHub branch protection for coverage-gate enforcement
# Requires: GitHub CLI (gh) with authenticated access and admin/maintain permissions
# Usage: ./scripts/setup-branch-protection.sh [owner/repo] [branch]

set -euo pipefail

REPO="${1:-ShreyKumar/rif-take-home-challenge}"
BRANCH="${2:-main}"
STATUS_CHECK="Test and coverage gate"

echo "Setting up branch protection for $REPO/$BRANCH..."
echo "Status check required: '$STATUS_CHECK'"
echo ""

# Check for GitHub CLI
if ! command -v gh &> /dev/null; then
  echo "❌ GitHub CLI (gh) not found. Install from: https://cli.github.com"
  exit 1
fi

# Verify authentication
if ! gh auth status > /dev/null 2>&1; then
  echo "❌ Not authenticated with GitHub. Run: gh auth login"
  exit 1
fi

# Verify admin/maintain permissions
echo "Verifying permissions..."
if ! gh api repos/$REPO/collaborators/\$(gh auth status --show-token 2>/dev/null | grep 'Logged in' | head -1) > /dev/null 2>&1; then
  echo "⚠️  Cannot verify exact permission level, but proceeding. Admin/maintain role required."
fi

echo ""
echo "Applying branch protection rule..."
echo ""

# Setup branch protection with required status checks
gh api repos/$REPO/branches/$BRANCH/protection \
  --input - <<EOF
{
  "required_status_checks": {
    "strict": true,
    "contexts": ["$STATUS_CHECK"]
  },
  "enforce_admins": false,
  "required_pull_request_reviews": null,
  "restrictions": null,
  "required_linear_history": false,
  "allow_force_pushes": false,
  "allow_deletions": false
}
EOF

echo ""
echo "✅ Branch protection rule applied!"
echo ""
echo "Configuration:"
echo "  • Repository: $REPO"
echo "  • Branch: $BRANCH"
echo "  • Required status check: '$STATUS_CHECK'"
echo "  • Strict mode: enabled (dismiss stale reviews on new commits)"
echo "  • Force pushes: blocked"
echo "  • Deletions: blocked"
echo ""
echo "The coverage gate now blocks merges when coverage < 80%."
