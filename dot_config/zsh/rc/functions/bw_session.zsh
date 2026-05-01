# bw_session / bw_lock — Bitwarden CLI のロック解除と解除のヘルパ。
#
# 旧版は ~/.config/zsh/bin/bw_session として PATH 経由で配布していたが、
# PATH 経由で外部実行されると子プロセス内で export しても親シェルに届かない
# 構造バグがあった。sheldon 経由で defer source される関数定義に切り替えた。
#
# 自動実行はしない（シェル起動毎の bw unlock プロンプトを避ける）。
# ただし `chezmoi apply/diff/update/verify` および `bw <data-cmd>` の zsh 関数
# ラッパが、tmpfs 上のキャッシュ経由で BW_SESSION を透過的に復元するため、
# 連続作業中にマスターパスワードを再入力する頻度は大幅に減る。
#
# キャッシュは `${XDG_RUNTIME_DIR:-/tmp}/bw_session_${UID}` (mode 0600)。
# 再起動でクリアされ、`bw_lock` で明示削除できる。

# --- internal cache helpers --------------------------------------------------

_bw_cache_file() {
    print -r -- "${XDG_RUNTIME_DIR:-/tmp}/bw_session_${UID:-$(id -u)}"
}

_bw_cache_clear() {
    local f
    f="$(_bw_cache_file)"
    [[ -e "$f" ]] && rm -f -- "$f"
    return 0
}

# Save the given session key to cache atomically with mode 0600.
_bw_cache_save() {
    local key="$1"
    [[ -z "$key" ]] && return 1
    local f tmp dir
    f="$(_bw_cache_file)"
    tmp="${f}.tmp.$$"
    dir="${f:h}"
    [[ -d "$dir" ]] || mkdir -p -- "$dir" 2>/dev/null || return 1
    (
        umask 077
        print -r -- "$key" > "$tmp"
    ) || { rm -f -- "$tmp" 2>/dev/null; return 1; }
    chmod 600 -- "$tmp" 2>/dev/null
    mv -f -- "$tmp" "$f" 2>/dev/null || { rm -f -- "$tmp" 2>/dev/null; return 1; }
    return 0
}

# Load cache into BW_SESSION. Verify with `bw unlock --check`.
# Silent on failure (no stdout/stderr). Removes invalid cache.
_bw_cache_load() {
    (( $+commands[bw] )) || return 1
    local f key
    f="$(_bw_cache_file)"
    [[ -r "$f" ]] || return 1
    key="$(<"$f" 2>/dev/null)" || return 1
    [[ -z "$key" ]] && { _bw_cache_clear; return 1; }
    if ! BW_SESSION="$key" command bw unlock --check >/dev/null 2>&1; then
        _bw_cache_clear
        return 1
    fi
    export BW_SESSION="$key"
    return 0
}

# Ensure BW_SESSION is set & valid in the current shell.
# Order: existing env (verified) -> cache -> interactive prompt.
# Returns 0 on success, non-zero if user cancelled / bw missing.
_bw_ensure_session() {
    if (( ! $+commands[bw] )); then
        echo "bw: bitwarden-cli (bw) is not installed" >&2
        return 1
    fi
    if [[ -n "$BW_SESSION" ]] && command bw unlock --check >/dev/null 2>&1; then
        return 0
    fi
    if _bw_cache_load; then
        return 0
    fi
    local key
    key="$(command bw unlock --raw)" || return $?
    [[ -z "$key" ]] && { echo "bw: failed to unlock vault" >&2; return 1; }
    export BW_SESSION="$key"
    _bw_cache_save "$key"
    return 0
}

# --- user-facing helpers -----------------------------------------------------

bw_session() {
    if (( ! $+commands[bw] )); then
        echo "bw_session: bitwarden-cli (bw) is not installed" >&2
        return 1
    fi

    if [[ "$1" == "-f" ]]; then
        local key
        key="$(command bw unlock --raw)" || return $?
        [[ -z "$key" ]] && { echo "bw_session: failed to unlock vault" >&2; return 1; }
        export BW_SESSION="$key"
        _bw_cache_save "$key"
        return 0
    fi

    _bw_ensure_session
}

bw_lock() {
    local had_session=0
    if [[ -n "$BW_SESSION" ]]; then
        command bw lock >/dev/null 2>&1
        unset BW_SESSION
        had_session=1
    fi
    _bw_cache_clear
    if (( had_session )); then
        echo "BW_SESSION cleared and vault locked"
    else
        echo "bw_lock: BW_SESSION was empty (cache cleared if any)"
    fi
}

# chezmoi_apply — 互換用シム。実体は下の `chezmoi` 関数ラッパに委譲。
# 既存の topgrade integration (`upd`) が呼ぶため API は維持する。
chezmoi_apply() {
    chezmoi apply "$@"
}

# --- command wrappers --------------------------------------------------------

# `chezmoi` 関数ラッパ。apply/diff/update/verify のみ BW_SESSION を確保してから
# 実 chezmoi に委譲する。それ以外のサブコマンドは触らず pass-through。
chezmoi() {
    if (( ! $+commands[chezmoi] )); then
        echo "chezmoi: not installed" >&2
        return 1
    fi
    case "$1" in
        apply|diff|update|verify)
            _bw_ensure_session || return $?
            ;;
    esac
    command chezmoi "$@"
}

# `bw` 関数ラッパ。lock/unlock/login/logout/status/config はキャッシュに触れず
# pass-through。データ系サブコマンド (get/list/sync/...) は cache から silent
# 復元を試みてから委譲 (失敗してもプロンプトは出さず実 bw に任せる)。
bw() {
    case "$1" in
        lock)
            command bw lock "${@:2}"
            local rc=$?
            unset BW_SESSION
            _bw_cache_clear
            return $rc
            ;;
        unlock|login|logout|status|config|completion|--help|-h|--version|"")
            command bw "$@"
            ;;
        *)
            if [[ -z "$BW_SESSION" ]]; then
                _bw_cache_load 2>/dev/null || true
            fi
            command bw "$@"
            ;;
    esac
}
