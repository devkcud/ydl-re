#!/usr/bin/env bash
set -Eeuo pipefail

fail() {
    printf '%s\n' "$*" >&2
    exit 1
}

version=${1:-}
if [[ $# != 1 || ! "$version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-(alpha|beta|rc)\.[1-9][0-9]*)?$ ]]; then
    fail 'Usage: bash tools/release.sh X.Y.Z[-alpha.N|-beta.N|-rc.N]'
fi

[[ $(go env GOHOSTOS)/$(go env GOHOSTARCH) == linux/amd64 ]] || fail 'Packaging requires native Linux/amd64'
root=$(git rev-parse --show-toplevel)
cd "$root"
[[ -z $(git status --porcelain --untracked-files=normal) ]] || fail 'Packaging requires a clean committed tree'

rev=$(git rev-parse HEAD)
stamp=$(git show -s --format=%ct "$rev")
out="$root/bin/releases/$version"
[[ ! -e "$out" && ! -L "$out" ]] || fail 'Release output already exists'

mkdir -p "$root/bin/releases"
stage=$(mktemp -d "$root/bin/releases/.build.XXXXXX")
trap 'rm -rf -- "$stage"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

name="yoinkdl_${version}_linux_amd64"
source="yoinkdl_${version}_source"
mkdir -p "$stage/out" "$stage/src" "$stage/pkg/$name"
git archive --format=tar --prefix="$source/" "$rev" | gzip -n > "$stage/out/$source.tar.gz"
tar -xzf "$stage/out/$source.tar.gz" -C "$stage/src"

pkg="$stage/pkg/$name"
(
    cd "$stage/src/$source"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v1 go build -trimpath -buildvcs=false \
        -ldflags "-s -w -X main.version=$version" -o "$pkg/yoinkdl" ./cmd/yoinkdl
    cp LICENSE SECURITY.md CHANGELOG.md "$pkg/"
    cp LICENSES/Go-BSD-3-Clause.txt "$pkg/LICENSE-Go"
)
actual=$("$pkg/yoinkdl" version)
[[ "$actual" == "yoinkdl $version" ]] || fail 'Packaged version does not match'
"$pkg/yoinkdl" download --help > /dev/null

printf 'Version: %s\nRevision: %s\nSource archive: %s.tar.gz\nToolchain: %s\n' \
    "$version" "$rev" "$source" "$(go version)" > "$pkg/SOURCE.txt"
printf '%s\n' "YoinkDL $version for Linux amd64" '' \
    'Run ./yoinkdl version or ./yoinkdl download --help.' \
    'Install with: mkdir -p "$HOME/.local/bin"' \
    'Then: install -m 0755 yoinkdl "$HOME/.local/bin/yoinkdl"' \
    'Ensure that directory is on PATH.' '' \
    'The project uses MPL 2.0; see LICENSE. The Go runtime license is in LICENSE-Go.' \
    "Matching sources are in $source.tar.gz alongside this archive on GitHub Releases." \
    'SOURCE.txt identifies the exact source revision and toolchain.' \
    'Documentation: https://github.com/devkcud/ydl-re' > "$pkg/README.txt"

tar --sort=name --mtime="@$stamp" --owner=0 --group=0 --numeric-owner --mode='u+rwX,go+rX,go-w' \
    -czf "$stage/out/$name.tar.gz" -C "$stage/pkg" "$name"
(
    cd "$stage/out"
    sha256sum "$name.tar.gz" "$source.tar.gz" > SHA256SUMS
    sha256sum -c SHA256SUMS
)

mv -T -n "$stage/out" "$out"
[[ ! -d "$stage/out" ]] || fail 'Release output appeared during packaging; preserved existing files'
printf 'Release artifacts: %s\n' "$out"
