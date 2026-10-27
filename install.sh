#!/usr/bin/env bash
# Usage: curl -fsSL https://raw.githubusercontent.com/FredySandoval/sho-rt/main/install.sh | bash
set -euo pipefail

REPO="FredySandoval/sho-rt"
BINARY="sho-rt"

info() { printf '%s\n' "$*" >&2; }
fail() { info "Error: $*"; exit 1; }
download() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --retry 3 "$1" -o "$2"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$2" "$1"
    else
        fail 'Install curl or wget first.'
    fi
}

main() {
    local os arch tmp version archive base expected actual destination
    case "$(uname -s)" in
        Linux) os=linux ;;
        Darwin) os=darwin ;;
        *) fail 'Only Linux and macOS are supported.' ;;
    esac
    case "$(uname -m)" in
        x86_64|amd64) arch=amd64 ;;
        aarch64|arm64) arch=arm64 ;;
        *) fail 'Only amd64 and arm64 are supported.' ;;
    esac
    command -v tar >/dev/null 2>&1 || fail 'Install tar first.'
    if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
        fail 'Install sha256sum or shasum first; checksum verification is required.'
    fi

    tmp=$(mktemp -d)
    # Keep the cleanup path available after main returns.
    trap "rm -rf -- $(printf '%q' "$tmp")" EXIT
    version=${VERSION:-}
    if [[ -z "$version" ]]; then
        download "https://api.github.com/repos/$REPO/releases/latest" "$tmp/release.json"
        version=$(awk -F '"' '/"tag_name"[[:space:]]*:/ {print $4; exit}' "$tmp/release.json")
    fi
    [[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$ ]] || fail "Invalid release version: $version"
    archive="${BINARY}_${version#v}_${os}_${arch}.tar.gz"
    base="https://github.com/$REPO/releases/download/$version"
    info "Downloading $BINARY $version for $os/$arch..."
    download "$base/$archive" "$tmp/$archive"
    download "$base/checksums.txt" "$tmp/checksums.txt"
    expected=$(awk -v name="$archive" '$2 == name {print $1}' "$tmp/checksums.txt")
    [[ "$expected" =~ ^[a-fA-F0-9]{64}$ ]] || fail "Missing or invalid checksum for $archive"
    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$tmp/$archive")
    else
        actual=$(shasum -a 256 "$tmp/$archive")
    fi
    [[ "${actual%% *}" == "$expected" ]] || fail 'Checksum mismatch; installation aborted.'
    tar -xzf "$tmp/$archive" -C "$tmp" "$BINARY"

    destination=${INSTALL_DIR:-}
    if [[ -z "$destination" ]]; then
        if [[ -w /usr/local/bin ]]; then
            destination=/usr/local/bin
        else
            destination="${HOME:?HOME must be set}/.local/bin"
        fi
    fi
    mkdir -p "$destination"
    # Stage on the destination filesystem, then replace atomically (safe for a running binary).
    local staged
    staged=$(mktemp "$destination/.sho-rt.XXXXXXXX")
    trap "rm -rf -- $(printf '%q' "$tmp"); rm -f -- $(printf '%q' "$staged")" EXIT
    cp "$tmp/$BINARY" "$staged"
    chmod 755 "$staged"
    mv -f "$staged" "$destination/$BINARY"
    info "Installed $destination/$BINARY"
    case ":${PATH:-}:" in
        *":$destination:"*) ;;
        *) info "Add this directory to PATH: $destination" ;;
    esac
    info 'Run sho-rt to open the dashboard. Restart an existing background service to use the new binary.'
}

main "$@"
