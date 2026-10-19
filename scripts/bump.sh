#!/usr/bin/env bash
# Sets a new version in mygo.json, commits it as v<version> with a tag of the
# same name, and pushes main and the tag, which makes the
# Release workflow build and publish it.
# Usage: scripts/bump.sh <version | patch | minor | major>
#
# The "## Unreleased" section of CHANGELOG.md becomes "## <version>" in the
# same commit, unless a "## <version>" section is there already: the update
# window shows it, it is the notes of the GitHub release, and the release
# fails without it. Pre-release versions such as 1.2.0-beta.1 are refused,
# as CCGuanjia has no beta channel yet.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
if [ $# -ne 1 ]; then
    sed -n '2,11s/^# \{0,1\}//p' "$0" >&2
    exit 1
fi

cd "$ROOT"
CURRENT="$(/usr/bin/python3 -c 'import json; print(json.load(open("mygo.json"))["version"])')"
# Prints the new version, or fails when the argument isn't a version or
# keyword or doesn't come after the current version.
VERSION="$(/usr/bin/python3 - "$CURRENT" "$1" <<'PY'
import re, sys

def parse(v):
    m = re.fullmatch(r"(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)", v)
    if not m:
        sys.exit(f"{v} isn't a version such as 1.2.0.")
    return tuple(int(x) for x in m.groups())

current, arg = parse(sys.argv[1]), sys.argv[2]
major, minor, patch = current
new = {
    "patch": (major, minor, patch + 1),
    "minor": (major, minor + 1, 0),
    "major": (major + 1, 0, 0),
}.get(arg) or parse(arg)
if new <= current:
    sys.exit("%d.%d.%d doesn't come after the current version %s." % (*new, sys.argv[1]))
print("%d.%d.%d" % new)
PY
)"
TAG="v$VERSION"

if [ "$(git symbolic-ref --short HEAD 2>/dev/null)" != main ]; then
    echo "Releases are made from main. Switch to it first." >&2
    exit 1
fi
if [ -n "$(git status --porcelain)" ]; then
    echo "The working tree has changes. Commit them first." >&2
    exit 1
fi
git fetch --quiet --tags origin main
if [ "$(git rev-parse HEAD)" != "$(git rev-parse origin/main)" ]; then
    echo "main differs from origin/main. Pull or push first." >&2
    exit 1
fi
if git rev-parse --verify --quiet "refs/tags/$TAG" >/dev/null \
    || [ -n "$(git ls-remote --tags origin "refs/tags/$TAG")" ]; then
    echo "Tag $TAG exists already." >&2
    exit 1
fi
if gh release view "$TAG" >/dev/null 2>&1; then
    echo "Release $TAG exists already." >&2
    exit 1
fi
# A secret the Release workflow uses but the repository lacks fails it
# halfway, or, for the update key, publishes a release no app updates to.
SECRETS="$(gh secret list --json name --jq '.[].name')" \
    || { echo "Couldn't list the repository's secrets." >&2; exit 1; }
MISSING="$(comm -23 \
    <(grep -o 'secrets\.[A-Z0-9_]*' .github/workflows/release.yml | sed 's/^secrets\.//' | sort -u) \
    <(sort -u <<<"$SECRETS"))"
if [ -n "$MISSING" ]; then
    echo "The Release workflow needs these secrets, which aren't set: $(echo $MISSING)" >&2
    echo "Set them with: gh secret set <name>" >&2
    exit 1
fi
if grep -q "^## $VERSION\$" CHANGELOG.md; then
    RENAME_UNRELEASED=
elif /usr/bin/python3 - <<'PY'
import re, sys
text = open("CHANGELOG.md").read()
m = re.search(r"^## Unreleased\n(.*?)(?=^## |\Z)", text, re.M | re.S)
sys.exit(0 if m and m.group(1).strip() else 1)
PY
then
    RENAME_UNRELEASED=1
else
    echo "CHANGELOG.md has no \"## $VERSION\" section, nor an \"## Unreleased\" one with notes. Add one and commit it first." >&2
    exit 1
fi

echo "==> $CURRENT -> $VERSION"
/usr/bin/python3 - "$VERSION" <<'PY'
import json, re, sys
# Replaces only the version, keeping the file's hand-written layout, which
# json.dump would spread over many lines.
with open("mygo.json") as f:
    text = f.read()
text, n = re.subn(r'^(  "version": ")[^"]*(")', r"\g<1>%s\g<2>" % sys.argv[1], text, count=1, flags=re.M)
if n != 1 or json.loads(text)["version"] != sys.argv[1]:
    sys.exit("Couldn't find the version in mygo.json.")
with open("mygo.json", "w") as f:
    f.write(text)
PY
FILES=mygo.json
if [ -n "$RENAME_UNRELEASED" ]; then
    /usr/bin/python3 - "$VERSION" <<'PY'
import re, sys
with open("CHANGELOG.md") as f:
    text = f.read()
text = re.sub(r"^## Unreleased$", "## " + sys.argv[1], text, count=1, flags=re.M)
with open("CHANGELOG.md", "w") as f:
    f.write(text)
PY
    FILES="CHANGELOG.md mygo.json"
fi
if [ "$(git diff --name-only | tr '\n' ' ')" != "$FILES " ] \
    || [ "$(git diff -U0 mygo.json | grep -c '^[-+] ')" != 2 ] \
    || { [ -n "$RENAME_UNRELEASED" ] && [ "$(git diff -U0 CHANGELOG.md | grep -c '^[-+]#')" != 2 ]; }; then
    echo "Expected only the version in $FILES to change:" >&2
    git diff >&2
    echo "Undo with: git checkout -- $FILES" >&2
    exit 1
fi
git commit --quiet -m "$TAG" $FILES
git tag "$TAG"
git --no-pager log -1 --stat --format='%h %s' "$TAG"
echo
echo "==> notes of the release, from CHANGELOG.md:"
scripts/release-notes.sh "$VERSION"
echo

git push --quiet --atomic origin main "$TAG"
echo "==> pushed $TAG. The Release workflow: https://github.com/sorrycc/CCGuanjia/actions/workflows/release.yml"
