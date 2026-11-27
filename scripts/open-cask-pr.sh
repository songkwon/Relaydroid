#!/bin/bash
set -euo pipefail

if [[ $# != 1 || ! "$1" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ || "$(uname -s)" != Darwin ]]; then
  echo "Usage on macOS: $0 vMAJOR.MINOR.PATCH" >&2
  exit 2
fi

tag="$1"
repo=songkwon/relaydroid
script_dir="$(cd "$(dirname "$0")" && pwd)"
repo_url="https://github.com/$repo.git"
cask=Casks/relaydroid.rb
branch="release/homebrew-$tag"
base="$(gh repo view "$repo" --json defaultBranchRef --jq .defaultBranchRef.name)"
test -n "$base"
account="$(gh api user --jq '[.id, .login] | join("+")')"
test -n "$account"

existing="$(gh pr list --repo "$repo" --base "$base" --head "$branch" --state open --json url --jq '.[0].url // empty')"
if [[ -n "$existing" ]]; then
  printf '%s\n' "$existing"
  exit 0
fi

stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
"$script_dir/verify-release-macos.sh" "$tag" "$stage/checksums.txt"
git clone --quiet --depth 1 --branch "$base" "$repo_url" "$stage/repo"
cd "$stage/repo"
python3 "$script_dir/update-cask.py" "$tag" "$stage/checksums.txt"
ruby -c "$cask"
if git diff --quiet -- "$cask"; then
  echo "The default branch already has this Homebrew update or a newer version."
  exit 0
fi

git add "$cask"
git -c user.name="${account#*+}" -c user.email="$account@users.noreply.github.com" \
  commit -m "Update Homebrew cask to $tag"
remote_ref="refs/heads/$branch"
existing_ref="$(git ls-remote --heads origin "$remote_ref")"
if [[ -n "$existing_ref" ]]; then
  git fetch --quiet --depth=2 origin "$remote_ref"
  test "$(git diff --name-only FETCH_HEAD^ FETCH_HEAD)" = "$cask"
  git show "FETCH_HEAD:$cask" | cmp - "$cask"
else
  git -c credential.helper= -c 'credential.helper=!gh auth git-credential' push origin "HEAD:$remote_ref"
fi
gh pr create --repo "$repo" --base "$base" --head "$branch" \
  --title "Update Homebrew cask to $tag" \
  --body "Update both macOS checksums after verifying the published app signatures, notarization tickets and architectures for $tag."
