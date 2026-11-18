#!/bin/sh
# =============================================================================
#  █████╗ ███████╗██╗   ██╗██╗  ██╗██╗   ██╗███████╗
# ██╔══██╗██╔════╝██║   ██║██║ ██╔╝██║   ██║██╔════╝
# ███████║███████╗██║   ██║█████╔╝ ██║   ██║███████╗
# ██╔══██║╚════██║██║   ██║██╔═██╗ ██║   ██║╚════██║
# ██║  ██║███████║╚██████╔╝██║  ██╗╚██████╔╝███████║
# ╚═╝  ╚═╝╚══════╝ ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝
#
#  Stream Anime From Your Terminal
#  https://github.com/hishantik/anilix
# =============================================================================

set -eu

REPO="hishantik/anilix"
BINARY="anilix"
BASE_URL="https://github.com/${REPO}/releases/download"
API_URL="https://api.github.com/repos/${REPO}/releases/latest"

# ─── Material Design 3 Color Palette ─────────────────────────────────────────
# Primary: #D0BCFF  |  Secondary: #CCC2DC  |  Tertiary: #EFB8C8
# Error: #F2B8B5   |  Success: #86D9A4     |  Warning: #F5C26B
# Surface: #1C2028 |  Text: #E6E1E5       |  Muted: #CAC4D0

ESC=$(printf '\033')

# RGB escape sequences for truecolor terminals
C_RESET="${ESC}[0m"
C_BOLD="${ESC}[1m"
C_DIM="${ESC}[2m"

# Primary purple palette
C_PRIMARY="${ESC}[38;2;208;188;255m"      # #D0BCFF
C_PRIMARY_BRIGHT="${ESC}[38;2;220;199;255m" # #DCC7FF
C_PRIMARY_DARK="${ESC}[38;2;142;107;205m"   # #8E6BCD

# Accent colors
C_SECONDARY="${ESC}[38;2;204;194;220m"    # #CCC2DC
C_TERTIARY="${ESC}[38;2;239;184;200m"    # #EFB8C8
C_PINK="${ESC}[38;2;217;70;239m"          # #D946EF
C_MAGENTA="${ESC}[38;2;157;78;221m"      # #9D4EDD

# Status colors
C_SUCCESS="${ESC}[38;2;134;217;164m"      # #86D9A4
C_WARNING="${ESC}[38;2;245;194;107m"     # #F5C26B
C_ERROR="${ESC}[38;2;242;184;181m"       # #F2B8B5

# Neutrals
C_SURFACE="${ESC}[38;2;28;32;40m"        # #1C2028 (background simulation)
C_TEXT="${ESC}[38;2;230;225;229m"         # #E6E1E5
C_MUTED="${ESC}[38;2;202;196;208m"        # #CAC4D0
C_OUTLINE="${ESC}[38;2;147;143;153m"      # #938F99

# Gradient for header (animated with sed)
GRADIENT_START="${C_PRIMARY}"
GRADIENT_END="${C_PINK}"

# ─── Animation Helpers ───────────────────────────────────────────────────────

# Spinner frames (Material Design inspired)
SPINNER_FRAMES='⠋ ⠙ ⠹ ⠸ ⠼ ⠴ ⠦ ⠧ ⠇ ⠏'

get_spinner_frame() {
    printf '%s' "$SPINNER_FRAMES" | cut -c$(( ($1 % 10) * 2 + 1 ))-$(($(($1 % 10)) * 2 + 2))
}

# Progress bar with Material Design gradient
draw_progress_bar() {
    pct="$1"
    [ "$pct" -gt 100 ] && pct=100
    width=40
    on=$(( pct * width / 100 ))
    off=$(( width - on ))

    # Gradient effect: use primary to pink transition
    bar="${C_PRIMARY}"
    for i in $(seq 1 "$on"); do
        # Shift color along gradient based on position
        if [ "$i" -lt $((width / 2)) ]; then
            bar="${bar}▌"
        else
            bar="${bar}▸"
        fi
    done
    bar="${bar}${C_MUTED}"
    for i in $(seq 1 "$off"); do
        bar="${bar}░"
    done

    # Draw bar with percentage
    printf "\r  ${bar} ${C_BOLD}%3d%%${C_RESET} " "$pct" >&2
}

# Animated text reveal
reveal_text() {
    text="$1"
    delay="${2:-0}"
    sleep "$delay"
    printf '%s' "$text"
}

# Typewriter effect for important messages
typewrite() {
    text="$1"
    delay="${2:-0.03}"
    for char in $(printf '%s' "$text" | sed 's/\(.\)/\1\n/g'); do
        printf '%s' "$char"
        sleep "$delay"
    done
}

# ─── Output Functions ────────────────────────────────────────────────────────

clear_line() {
    printf '\r\033[K' >&2
}

println() {
    printf '\n' >&2
}

separator() {
    printf '\n  %b%s%b\n' "$C_OUTLINE" "$(printf '─%.0s' $(seq 1 56))" "$C_RESET" >&2
}

# ─── Header Banner ───────────────────────────────────────────────────────────

print_banner() {
    # Animated gradient banner
    BANNER_LINES="
  ${C_PRIMARY_BRIGHT} █████╗ ${C_PINK} ███████╗${C_PRIMARY}██╗   ██╗${C_TERTIARY}██╗  ██╗${C_PINK}██╗   ██╗${C_PRIMARY}███████╗${C_RESET}
  ${C_PRIMARY_BRIGHT}██╔══██╗${C_PINK}██╔════╝${C_PRIMARY}██║   ██║${C_TERTIARY}██║ ██╔╝${C_PINK}██║   ██║${C_PRIMARY}██╔════╝${C_RESET}
  ${C_PRIMARY_BRIGHT}███████║${C_PINK}███████╗${C_PRIMARY}██║   ██║${C_TERTIARY}█████╔╝ ${C_PINK}██║   ██║${C_PRIMARY}███████╗${C_RESET}
  ${C_PRIMARY_BRIGHT}██╔══██║${C_PINK}╚════██║${C_PRIMARY}██║   ██║${C_TERTIARY}██╔═██╗ ${C_PINK}██║   ██║${C_PRIMARY}╚════██║${C_RESET}
  ${C_PRIMARY_BRIGHT}██║  ██║${C_PINK}███████║${C_TERTIARY}╚██████╔╝${C_PINK}██║  ██╗${C_TERTIARY}╚██████╔╝${C_PINK}███████║${C_RESET}
  ${C_PRIMARY_BRIGHT}╚═╝  ╚═╝${C_PINK}╚══════╝${C_TERTIARY} ╚═════╝ ${C_PINK} ╚═╝  ╚═╝ ${C_TERTIARY} ╚═════╝ ${C_PINK}╚══════╝${C_RESET}
"
    printf '%s\n' "$BANNER_LINES" >&2

    # Subtitle with typewriter effect
    printf '\n  ' >&2
    typewrite "${C_MUTED}Stream Anime From Your Terminal${C_RESET}" 0.02
    println
    separator
}

# ─── Status Messages ─────────────────────────────────────────────────────────

info() {
    clear_line
    printf '  %b◆%b %s\n' "$C_PRIMARY" "$C_RESET" "$*" >&2
}

success() {
    clear_line
    printf '  %b✓%b %s\n' "$C_SUCCESS" "$C_RESET" "$*" >&2
}

warning() {
    clear_line
    printf '  %b⚠%b %s\n' "$C_WARNING" "$C_RESET" "$*" >&2
}

error() {
    clear_line
    printf '  %b✗%b %s\n' "$C_ERROR" "$C_RESET" "$*" >&2
}

progress() {
    clear_line
    printf '  %b⟩%b %s ' "$C_PRIMARY" "$C_RESET" "$*" >&2
}

# ─── Loading Spinner ─────────────────────────────────────────────────────────

spin() {
    msg="$1"
    delay="${2:-0.08}"
    i=0
    while kill -0 "$curl_pid" 2>/dev/null; do
        frame=$(get_spinner_frame $i)
        printf '\r  %b%b%b %s' "$C_PRIMARY" "$frame" "$C_RESET" "$msg" >&2
        i=$((i + 1))
        sleep "$delay"
    done
    # Clear spinner
    printf '\r\033[K' >&2
    info "$msg"
}

# ─── Helpers ─────────────────────────────────────────────────────────────────

need_cmd() {
    command -v "$1" >/dev/null 2>&1 || die "Required command '$1' not found. Please install it first."
}

die() {
    error "$*"
    exit 1
}

# ─── Cleanup ─────────────────────────────────────────────────────────────────

TMPDIR=""
cleanup() {
    [ -n "$TMPDIR" ] && rm -rf "$TMPDIR"
}
trap cleanup EXIT

# ─── System Detection ────────────────────────────────────────────────────────

detect_os() {
    os="$(uname -s)"
    case "$os" in
        Linux*)     echo "linux" ;;
        Darwin*)    echo "darwin" ;;
        CYGWIN*|MINGW*|MSYS*) echo "windows" ;;
        *)          die "Unsupported OS: $os" ;;
    esac
}

detect_arch() {
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *)            die "Unsupported architecture: $arch" ;;
    esac
}

is_termux() {
    case "${PREFIX:-}" in *com.termux*) return 0;; esac
    [ -n "${TERMUX_VERSION:-}" ] && return 0
    return 1
}

detect_install_dir() {
    if is_termux; then
        echo "${PREFIX}/bin"
        return
    fi

    if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
        echo "/usr/local/bin"
        return
    fi

    if [ -d /usr/local ]; then
        if [ -w /usr/local ]; then
            mkdir -p /usr/local/bin
            echo "/usr/local/bin"
            return
        fi
        if command -v sudo >/dev/null 2>&1; then
            sudo mkdir -p /usr/local/bin 2>/dev/null && \
            sudo test -w /usr/local/bin && \
            echo "/usr/local/bin" && return
        fi
    fi

    mkdir -p "$HOME/.local/bin"
    echo "$HOME/.local/bin"
}

# ─── Get Latest Version ──────────────────────────────────────────────────────

get_latest_version() {
    need_cmd curl
    json=$(curl -fsSL "$API_URL" 2>/dev/null)
    if command -v jq >/dev/null 2>&1; then
        tag=$(printf '%s' "$json" | jq -r '.tag_name // empty')
    else
        tag=$(printf '%s' "$json" | grep '"tag_name"' | head -1 | sed -E 's/.*"tag_name":\s*"([^"]+)".*/\1/')
    fi
    [ -z "$tag" ] && die "Failed to fetch latest version from GitHub"
    echo "$tag"
}

# ─── Download with Animated Progress ────────────────────────────────────────

download_file() {
    url="$1"
    output="$2"

    # Start download in background
    curl -fSL "$url" -o "$output" -# 2>/dev/null &
    curl_pid=$!

    # Animated progress bar
    pct=0
    while kill -0 "$curl_pid" 2>/dev/null; do
        draw_progress_bar "$pct"
        # Ease-in-out animation curve
        if [ "$pct" -lt 30 ]; then
            pct=$((pct + 2))
        elif [ "$pct" -lt 70 ]; then
            pct=$((pct + 3))
        elif [ "$pct" -lt 95 ]; then
            pct=$((pct + 1))
        else
            pct=$((pct + 1))
        fi
        [ "$pct" -gt 99 ] && pct=99
        sleep 0.12
    done

    wait "$curl_pid"
    exit_code=$?

    # Complete the bar
    draw_progress_bar 100
    clear_line

    return $exit_code
}

# ─── Check Dependencies ──────────────────────────────────────────────────────

check_deps() {
    os="${1:-}"
    has_player=false

    if command -v mpv >/dev/null 2>&1; then
        has_player=true
        player=$(command -v mpv)
    elif command -v vlc >/dev/null 2>&1; then
        has_player=true
        player=$(command -v vlc)
    elif command -v iina >/dev/null 2>&1; then
        has_player=true
        player=$(command -v iina)
    fi

    if [ "$has_player" = false ]; then
        warning "No supported video player found. Anilix needs one of: mpv, vlc, iina"
        printf '\n'
        printf '  %b┌─ Install Guide ───────────────────────────────────────┐%b\n' "$C_OUTLINE" "$C_RESET" >&2
        printf '  %b│%b\n' "$C_OUTLINE" "$C_RESET" >&2

        case "$os" in
            termux)
                printf '  %b│%b   %b pkg install mpv%b\n' "$C_OUTLINE" "$C_RESET" "$C_PRIMARY" "$C_RESET" >&2
                ;;
            darwin)
                printf '  %b│%b   %b brew install mpv%b      (recommended)\n' "$C_OUTLINE" "$C_RESET" "$C_PRIMARY" "$C_RESET" >&2
                printf '  %b│%b   %b brew install --cask iina%b\n' "$C_OUTLINE" "$C_RESET" "$C_PRIMARY" "$C_RESET" >&2
                ;;
            windows)
                printf '  %b│%b   %b winget install mpv%b\n' "$C_OUTLINE" "$C_RESET" "$C_PRIMARY" "$C_RESET" >&2
                printf '  %b│%b   or download from: https://mpv.io/installation/\n' "$C_OUTLINE" "$C_RESET" >&2
                ;;
            linux|*)
                command -v apt >/dev/null 2>&1 && printf '  %b│%b   %b sudo apt install mpv%b\n' "$C_OUTLINE" "$C_RESET" "$C_PRIMARY" "$C_RESET" >&2
                command -v dnf >/dev/null 2>&1 && printf '  %b│%b   %b sudo dnf install mpv%b\n' "$C_OUTLINE" "$C_RESET" "$C_PRIMARY" "$C_RESET" >&2
                command -v pacman >/dev/null 2>&1 && printf '  %b│%b   %b sudo pacman -S mpv%b\n' "$C_OUTLINE" "$C_RESET" "$C_PRIMARY" "$C_RESET" >&2
                command -v apk >/dev/null 2>&1 && printf '  %b│%b   %b sudo apk add mpv%b\n' "$C_OUTLINE" "$C_RESET" "$C_PRIMARY" "$C_RESET" >&2
                ;;
        esac

        printf '  %b│%b\n' "$C_OUTLINE" "$C_RESET" >&2
        printf '  %b└───────────────────────────────────────────────────────┘%b\n' "$C_OUTLINE" "$C_RESET" >&2
        printf '\n'
    else
        success "Found player: $player"
    fi
}

# ─── Install ────────────────────────────────────────────────────────────────

do_install() {
    version="${1:-}"
    os=$(detect_os)
    arch=$(detect_arch)
    install_dir=$(detect_install_dir)

    separator
    printf '\n'

    if [ -z "$version" ]; then
        progress "Fetching latest version..."
        version=$(get_latest_version)
        clear_line
        info "Latest version: ${C_BOLD}${version}${C_RESET}"
    fi

    printf '\n  %b%b Fetching ${C_BOLD}%s${C_RESET}%b for ${C_PRIMARY}%s${C_RESET}/${C_TERTIARY}%s${C_RESET}...\n\n' "$C_PRIMARY" "$SPINNER_FRAMES" "$version" "$C_PRIMARY" "$os" "$arch"

    # Build download URL
    case "$os" in
        windows)
            ext="zip"
            archive_name="${BINARY}_${os}_${arch}.${ext}"
            ;;
        termux)
            ext="tar.gz"
            archive_name="${BINARY}_termux_${arch}.${ext}"
            ;;
        *)
            ext="tar.gz"
            archive_name="${BINARY}_${os}_${arch}.${ext}"
            ;;
    esac

    url="${BASE_URL}/${version}/${archive_name}"

    # Create temp directory
    TMPDIR=$(mktemp -d)

    progress "Downloading..."
    printf '\n'
    printf '  %b%b %s%b\n' "$C_MUTED" "$C_OUTLINE" "$url" "$C_RESET" >&2

    if ! download_file "$url" "${TMPDIR}/${archive_name}"; then
        printf '\n'
        die "Download failed. Check if version ${version} exists:\n  ${C_MUTED}https://github.com/${REPO}/releases${C_RESET}"
    fi

    printf '\n  '
    info "Extracting archive..."

    orig_dir=$(pwd)
    cd "$TMPDIR"
    if [ "$os" = "windows" ]; then
        need_cmd unzip
        unzip -qo "$archive_name"
    else
        tar -xzf "$archive_name"
    fi
    cd "$orig_dir"

    if [ ! -f "${TMPDIR}/${BINARY}" ]; then
        die "Binary '${BINARY}' not found in archive"
    fi

    chmod +x "${TMPDIR}/${BINARY}"

    printf '\n  '
    info "Installing to ${install_dir}..."

    if [ -w "$install_dir" ]; then
        cp "${TMPDIR}/${BINARY}" "${install_dir}/${BINARY}"
    elif command -v sudo >/dev/null 2>&1; then
        sudo cp "${TMPDIR}/${BINARY}" "${install_dir}/${BINARY}"
    else
        die "No write permission to ${install_dir} and sudo not available"
    fi

    printf '\n'
    separator
    printf '\n'
    success "Installed ${C_BOLD}${install_dir}/${BINARY}${C_RESET}"
    printf '\n'

    # PATH warning
    case ":${PATH}:" in
        *":${install_dir}:"*) ;;
        *)
            printf '  %bℹ%b ${C_MUTED}%s is not in your PATH.%b\n' "$C_PRIMARY" "$C_RESET" "$install_dir" "$C_RESET" >&2
            printf '\n'
            printf '  %bAdd to your shell profile (.bashrc, .zshrc, etc.):%b\n' "$C_MUTED" "$C_RESET" >&2
            printf '\n'
            printf '    %bexport PATH="%s:$PATH"%b\n' "$C_PRIMARY" "$install_dir" "$C_RESET" >&2
            printf '\n'
            ;;
    esac

    # Dependency check
    check_deps "$os"

    separator
    printf '\n'
    printf '  %b▶ Run:%b %banilix%b\n' "$C_TERTIARY" "$C_RESET" "$C_BOLD" "$C_RESET" >&2
    printf '\n'
}

# ─── Update ──────────────────────────────────────────────────────────────────

do_update() {
    print_banner
    info "Updating ${BINARY}..."
    printf '\n'
    do_install ""
}

# ─── Uninstall ───────────────────────────────────────────────────────────────

do_uninstall() {
    print_banner
    install_dir=$(detect_install_dir)
    binary_path="${install_dir}/${BINARY}"

    if [ ! -f "$binary_path" ]; then
        die "${BINARY} not found at ${binary_path}"
    fi

    printf '\n  '
    info "Removing ${binary_path}..."
    rm -f "$binary_path"

    printf '\n'
    success "Uninstalled ${binary_path}"
    printf '\n'
}

# ─── Usage ───────────────────────────────────────────────────────────────────

usage() {
    cat <<EOF

  ${C_PRIMARY_BRIGHT}Anilix Installer${C_RESET} — Stream Anime From Your Terminal

  ${C_MUTED}Usage:${C_RESET}
    ${C_PRIMARY}curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | sh${C_RESET}
    ${C_PRIMARY}sh install.sh [options]${C_RESET}

  ${C_MUTED}Options:${C_RESET}
    ${C_PRIMARY}--help${C_RESET}          Show this help message
    ${C_PRIMARY}--update${C_RESET}         Update to the latest version
    ${C_PRIMARY}--uninstall${C_RESET}      Remove the installed binary
    ${C_PRIMARY}--version VER${C_RESET}    Install a specific version (e.g. --version v1.0.0)

  ${C_MUTED}Examples:${C_RESET}
    ${C_PRIMARY}curl -fsSL ... | sh${C_RESET}                    Install latest
    ${C_PRIMARY}sh install.sh --update${C_RESET}                  Update
    ${C_PRIMARY}sh install.sh --version v1.0.0${C_RESET}           Install specific version
    ${C_PRIMARY}sh install.sh --uninstall${C_RESET}                Uninstall

EOF
}

# ─── Main ────────────────────────────────────────────────────────────────────

main() {
    action="install"
    version=""

    # Check for piped input (curl | sh)
    if [ -t 0 ]; then
        # Interactive mode - show banner
        print_banner
    fi

    while [ $# -gt 0 ]; do
        case "$1" in
            --help|-h)
                usage
                exit 0
                ;;
            --update|-u)
                action="update"
                shift
                ;;
            --uninstall)
                action="uninstall"
                shift
                ;;
            --version|-v)
                [ $# -lt 2 ] && die "--version requires a value (e.g. --version v1.0.0)"
                version="$2"
                action="install"
                shift 2
                ;;
            *)
                die "Unknown option: $1. Run with --help for usage."
                ;;
        esac
    done

    case "$action" in
        install)   do_install "$version" ;;
        update)    do_update ;;
        uninstall) do_uninstall ;;
    esac
}

main "$@"
