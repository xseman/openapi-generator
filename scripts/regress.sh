#!/usr/bin/env bash
# Show how the working tree changes generated output, compared to any git ref.
#
#   scripts/regress.sh            # working tree vs HEAD
#   scripts/regress.sh master     # working tree vs master
#   scripts/regress.sh v0.1.1     # working tree vs a tag
#
# Every spec in testdata/specs/feature goes through every generator in
# templates/, once per build and option set. Nothing is stored between runs:
# the "before" side is rebuilt from git on demand. Exits 1 if anything differs,
# after printing the diff.
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

ref=${1:-HEAD}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "building working tree..."
go build -o "$tmp/new-bin" ./cmd/openapi-generator

# git archive, not git worktree/checkout: read-only, leaves the repo untouched.
echo "building $ref..."
mkdir -p "$tmp/src"
git archive "$ref" | tar -x -C "$tmp/src"
(cd "$tmp/src" && go build -o "$tmp/old-bin" ./cmd/openapi-generator)

# Boolean options flipped from their defaults (see `config-help <generator>`).
# Each generator runs once with its defaults and once per option flipped on its
# own, so template branches behind an option are rendered too. One at a time:
# useSingleRequestParameter=false would hide what prefixParameterInterfaces does.
declare -A flipped=(
    [typescript-fetch]="withPackageJson=true withInterfaces=true useSingleRequestParameter=false prefixParameterInterfaces=true withoutRuntimeChecks=true stringEnums=true validationAttributes=true"
    [dart-fetch]="useDartIoSender=false"
)

# run GEN OUT [ARGS...]: generate $spec with both builds into */OUT/$name.
# -t hands each build its own tree's templates. Left to itself, a binary run
# from the repo root prefers ./templates over its embedded copy, and both sides
# would render the working tree's: template changes would never show.
# --skip-validate-spec on both sides: we are diffing output, not validating input.
run() {
    local gen=$1 out=$2
    shift 2
    "$tmp/old-bin" generate -i "$spec" -g "$gen" -t "$tmp/src/templates/$gen" -o "$tmp/old/$out/$name" --skip-validate-spec "$@" >/dev/null 2>&1 || echo "  ! $ref failed: $out $name"
    "$tmp/new-bin" generate -i "$spec" -g "$gen" -t "templates/$gen" -o "$tmp/new/$out/$name" --skip-validate-spec "$@" >/dev/null 2>&1 || echo "  ! working tree failed: $out $name"
}

for spec in testdata/specs/feature/*.yaml; do
    name=$(basename "$spec" .yaml)
    for dir in templates/*/; do
        gen=$(basename "$dir")
        run "$gen" "$gen"
        for opt in ${flipped[$gen]:-}; do run "$gen" "$gen-$opt" -p "$opt"; done
    done
done

# -I: README.md (withPackageJson) stamps the generation time.
if (cd "$tmp" && diff -ruN -I '^- Build date: ' old new); then
    echo "no change in generated output vs $ref"
else
    echo
    echo "generated output differs from $ref (see diff above)"
    exit 1
fi
