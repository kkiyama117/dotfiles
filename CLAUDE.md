# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

[Chezmoi](https://www.chezmoi.io/) で管理する個人用 dotfiles リポジトリ。主要ターゲットは Linux (Manjaro)、副次的に Windows / WSL。Bitwarden をシークレットバックエンドとして利用する。

## Common Commands

### Chezmoi
- `chezmoi diff` — 適用前にローカルとの差分を確認する（編集後は必ず実行）
- `chezmoi apply` — このリポジトリの変更を `~/` 以下に反映
- `chezmoi update` — `git pull` + `apply`
- `chezmoi cd` — このリポジトリ (`~/.local/share/chezmoi`) に移動
- `chezmoi add ~/path` — 既存ファイルを管理対象に追加（prefix 変換は自動）

### Bootstrap (新規マシン)
```
chezmoi init --apply kkiyama117
```
初回のみ `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl` が走り、Manjaro 上では `rustup`, `mise`, `paru`, および `ttf-plemoljp-bin / fcitx5 / neovim / wezterm / ripgrep / pueue / zoxide` 等の paru パッケージを導入する。Manjaro 以外では即 `exit 0`。後にnix経由に置き変える.

### Bitwarden セッション
`apply` 時に pre-source hook (`.executable_password_manager.sh`) が `bw` の存在を確認し、テンプレ展開でマスターパスワードが要求される。`rc/functions/bw_session.zsh` で `chezmoi` / `bw` の zsh 関数ラッパが定義されており、`chezmoi apply/diff/update/verify` および `bw <データ系サブコマンド>` 実行時に **tmpfs 上のキャッシュ (`${XDG_RUNTIME_DIR}/bw_session_${UID}`, mode 0600, 再起動でクリア)** から `BW_SESSION` を透過的に復元するため、連続実行で再入力を求められない:
```
chezmoi diff            # 初回はマスターパスワード入力 → cache に保存
chezmoi apply           # 同一マシンの別シェルでも cache 経由で再利用
bw get item <id>        # data 系サブコマンドも cache から silent restore
```
明示的に解錠したい場合は `bw_session` (cache 優先、なければプロンプト) または `bw_session -f` (強制再解錠)。終了時に確実に破棄したい場合は `bw_lock` で `unset BW_SESSION` + `bw lock` + cache 削除を一括実行する。シェル起動時には何も走らないので、auto-restore による意図しない `bw` 呼び出しは発生しない。

`BW_SESSION` は Bitwarden ボルト全体へのアクセスキーで、子プロセスに継承される。tmpfs cache は再起動で消えるが、長時間放置するマシンでは作業終了時に `bw_lock` を推奨。

### Xrdp (該当時)
`/etc/xrdp/startwm.sh` を編集して `$XDG_CONFIG_HOME/zsh/.zprofile` を読み込ませる必要がある。

## Architecture

### Path 規約（chezmoi の prefix）
- `dot_*` → `~/.*`  (例: `dot_config/` → `~/.config/`)
- `private_*` → 0600 で配置
- `empty_*` → 空ファイルの場合必須
- `executable_*` → 実行ビット付き
- `symlink_*` → シンボリックリンクとして配置（中身は target path）
- `*.tmpl` → Go `text/template` で展開後に配置

### 重要なシンボリックリンク
- `symlink_dot_claude` の中身は `.config/claude` → 結果として **`~/.claude` は `~/.config/claude` への symlink**。`~/.claude/CLAUDE.md` 等を編集すると、このリポジトリの `dot_config/claude/` 配下が更新される（chezmoi の管理対象は後者）。

### テンプレート分岐の軸
- `eq .chezmoi.os "linux" / "windows"`
- `.chezmoi.osRelease.id` (例: `manjaro`)
- `.chezmoi.kernel.osrelease | lower | contains "microsoft"` で WSL 判定
- 共有スニペットは `.chezmoitemplates/{linux,windows}/` 配下

### Bitwarden 参照（`*.tmpl` 内）
- `{{ (bitwarden "item" "<id>").value }}`
- `{{ (bitwardenFields "item" "<id>").<field>.value }}`

代表例: `dot_config/git/config.tmpl` で `user.name` / `user.email` / `user.signingkey` を Bitwarden から注入。

### ZSH ロード順
- ルート直下の `dot_zshenv` (= `~/.zshenv`) で `XDG_CONFIG_HOME` 等を確定し、`ZDOTDIR` を `~/.config/zsh` に向ける
- 以降は `dot_config/zsh/{dot_zshenv,dot_zprofile,dot_zshrc}` と `rc/*.zsh` が読まれる
- プラグインは Sheldon (`dot_config/sheldon/plugins.toml`) で `zsh-defer` 経由の遅延読込
- mise は sheldon 内で `eval "$(mise activate zsh)"` により有効化

### `.chezmoiignore` の特徴
- ターゲット側にコピーしないものを列挙: `README.md`, `CLAUDE.md`, `*.code-workspace`, `*.zwc`, `dot_config/chezmoi/*` (除く `chezmoi.toml`), `dot_config/gh/hosts.yml`, `.local/share/rye/*` (除く `config.toml`), `.local/share/gnupg/*` (除く `common.conf`)
- OS 別: 非 Windows では `AppData` を無視、非 Linux では `.config/app/file.conf` を無視

## Workflow Notes
- 編集後は **必ず `chezmoi diff` で差分確認 → `chezmoi apply`**。テンプレ構文ミスはここで初めて顕在化する。
- 新規 Bitwarden 参照を加えたら、別シェルで `BW_SESSION` を解いて `chezmoi apply` を実行し、展開結果を確認する。
