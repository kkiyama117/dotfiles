# CHANGELOG

このリポジトリは厳密なバージョニングを行わない dotfiles 管理リポジトリ。
変更履歴は [Keep a Changelog](https://keepachangelog.com/) に倣って **日付 + スコープ** でまとめる。
細粒度な diff は `git log` を参照。

---

## 2026-04-29 — kitty 統合 / Claude × tmux チートシート

### Added
- `dot_config/kitty/kitty.conf` を新規作成。ghostty と font / opacity / blur / キーマップ全クリアを揃える（commit `cfaf8f0`）
- `docs/claude_tmux_cheatsheet.md` を新規作成。claude コマンド・prefix + C 二段ショートカット・典型フロー・status-bar・通知設定・trouble-shoot 早見表を1ファイルに集約（commit `152e37c`）

### Changed
- tmux: kitty 主運用への移行に伴い `set -g focus-events on` を再有効化（resurrect / nvim autoread / pane border 動的更新の恩恵を取り戻す）。Ghostty + X11 + GTK DE 併用時のバグ (ghostty-org/ghostty#9620) は kitty では再現しないため off に戻す必要なし（commit `070f998`）
- `docs/keybinds.md` / `docs/manage_claude.md` / `docs/claude_tmux_cheatsheet.md`: focus-events on 前提に追従、Ghostty 併用時の暫定対処を併記
- `README.md`: ターミナルエミュレータ一覧を `wezterm` から `kitty` に置換（ghostty と併記）

---

## 2026-04-29 — Claude cockpit (tmux 統合)

### Added
- **prefix + C 二段プレフィックス（claude_table）** を新規作成（commit `22fe7c8`）
  - `c` … 現 pane で `claude --continue`
  - `n` … fzf で branch 選択 → 新規 worktree + 2-pane claude session を起動
  - `r` … 現 session 内の claude pane を kill → `--continue` で再起動
  - `s` … `claude-*` 名の session を fzf で選び switch
  - `k` … 現 session が `claude-*` なら session + worktree を一括削除（要 y/n 確認）
- 直下バインド: `prefix + g`（worktree picker）/ `prefix + s`（全 session picker）
- ヘルパースクリプト群 `dot_config/tmux/scripts/`（`tmux-claude-new.sh`, `claude-pick-branch.sh`, `claude-pick-session.sh`, `claude-respawn-pane.sh`, `claude-kill-session.sh`, `claude-status-count.sh`, `claude-branch.sh`, `tpm-bootstrap.sh`）
- zsh ラッパー関数 `tmux_claude_new <branch>`（`dot_config/zsh/rc/my_plugins/tmux.zsh`）
- TPM 連携: `tmux-resurrect` + `tmux-continuum` (15 分自動保存) + `tmux-yank`
- ステータスバー: `[claude:N]`（実行中 claude プロセス数）+ 現 pane の git branch、`pane_current_command == claude` の pane を黄色枠でハイライト
- 通知 / サウンドフック `dot_local/bin/executable_claude-notify-sound.sh`: `Notification` / `Stop` / `subagent-stop` / `error` で別音 + wired-notify popup（`--expire-time=0` でクリックまで居残る）
- ドキュメント `docs/manage_claude.md`（複数 Claude Code セッション管理ガイド）

### Fixed
- prefix を `C-b` (tmux default) に確定（commit `a25a1fc`）。過去に試した C-t / C-Space / C-\ / C-] / F12 / C-a への変更はすべて、Ghostty + X11 + GTK DE での `focus-events` 暴発を SKK 干渉と誤認した早合点だった（ghostty-org/ghostty#9620）

---

## 2026-04-29 — zsh / mise / sheldon セキュリティ & クリーンアップレビュー

zsh 起動シーケンス、Bitwarden 統合、mise 設定、sheldon プラグイン管理に対する一斉レビュー（旧 `docs/todos.md` C-1 ~ L-9 / F-1 / F-2）。
詳細な背景・対応理由は git log（commit `7cd0cb0` / `39ec75a` / `4424716` / `ee5108c` / `7399c71` 周辺）を参照。

### Security — Critical
- **C-1**: `OPENROUTER_API_KEY` を `~/.zshenv` から `dot_config/zsh/rc/private_secrets.zsh.tmpl`（0600）に分離。zshenv 経由の全 zsh プロセスへの export と `/proc/<pid>/environ` 観測リスクを排除
- **C-2**: `bw_session` を auto-execute スクリプトから `rc/functions/bw_session.zsh` の defer-source される関数定義に置き換え。対の `bw_lock`（`unset BW_SESSION`）も追加。`apply = ['path']` 由来の export-not-propagated 構造バグも解消

### Security — High
- **H-1**: Bitwarden item UUID をリポジトリから除去。`.chezmoi.toml.tmpl` の `promptStringOnce` で `bitwardenOpenRouterItemId` を `~/.config/chezmoi/chezmoi.toml` に保存（`.chezmoiignore` で deploy 対象外）。`promptStringOnce` を当初 `dot_config/chezmoi/chezmoi.toml.tmpl` に置いたが function not defined で失敗 → ソースルート移動で解消（commit `7399c71`）
- **H-3**: sheldon プラグインに pin を導入。`zsh-defer` (rev), `zsh-autosuggestions` (v0.7.1), `zsh-completions` (0.36.0), `fast-syntax-highlighting` (auscompgeek v1.55), `tmux-xpanes` (v4.2.0)
- **H-4**: mise を `paranoid = true` / `yes = false` / `not_found_auto_install = false` に変更。タイポスクワッティング・チェックサム省略リスクを軽減

### Fixed — High（動作バグ）
- **H-2**: `eval $(keychain ... $GPG_KEY_ID)` の引用符抜けを `eval "$(keychain -q --eval --agents ssh,gpg "$_gpg_key_id")"` に修正。`$GPG_KEY_ID` 未定義問題は `git config --global user.signingkey` から実行時取得で解決
- **H-5**: `bw_session` の PATH 経由実行による export-not-propagated バグを C-2 で同時解消
- **H-6**: `for_development.zsh:33` の `$home`（小文字）→ `$HOME` に修正
- **H-7**: `for_development.zsh` の二重 export（GMS_PATH / BOOST_ROOT / LDFLAGS / CPPFLAGS）整理、`ANDROID_JAVA_HOME` の絶対パス化、`pkgconfig` を `LD_LIBRARY_PATH` から `PKG_CONFIG_PATH` へ移行
- **H-8**: `WORDCHARS=${WORDCHARS//\/[&.;]}` の置換パターンを `${WORDCHARS//[\/&.;]/}` に修正
- **H-9**: `[[ -v "$DEBUG" ]]` のロジックバグ（`-v` の引数は変数名）を `[[ -v DEBUG ]]` に修正（`dot_zshenv.tmpl` / `dot_zshrc` 両方）
- **H-10**: 未定義関数 `dir_color` の呼び出しを `dircolors -b` の inline 化（`commands[dircolors]` ガード付き）に置換
- **H-11**: `mkdir $XDG_DATA_HOME/zsh` に `-p` 追加
- **H-12**: `zsh-syntax-highlighting` と `fast-syntax-highlighting` の重複ロードを解消（`fast-syntax-highlighting` のみに集約）

### Fixed — Medium
- **M-2**: mise の `env_file = '.env'` をコメントアウト。サードパーティ repo に cd する際の環境汚染を防止
- **M-3**: `gpg-connect-agent` 呼び出し 2 箇所に `(( $+commands[gpg-connect-agent] ))` ガード追加
- **M-4**: `~/.local/bin/env` の無条件 source を削除（chezmoi 管理外、`INSTALLER_NO_MODIFY_PATH=1` 設定済みで冗長）
- **M-5**: `cargo/env` の冗長 source を削除（`$CARGO_HOME/bin` は `dot_zshenv.tmpl` で PATH 追加済み）
- **M-6**: `dot_zprofile` の `GTK2_RC_FILES` 二重定義を `dot_zshenv.tmpl` に一元化、`BROWSER` を `chrome / chromium / firefox` の動的 fallback に変更、`ZSH_VERSION` 判定の冗長ブロックを削除（commit `4424716`）
- **M-7**: `cd` 関数の二重再定義を `onefetch.zsh` 側で `chpwd_functions` に切り替え、グローバル `LAST_REPO` を `__ONEFETCH_LAST_REPO` にリネーム（commit `4424716`）
- **M-8**: `.chezmoiignore` のサンプル残骸（`*.txt`, `backups/` 等）を整理、重複していた `/docs/**/*` を `docs/` に統合
- **M-9**: `dot_zshenv` がディレクトリ扱いされていた問題を解消。`for_development.zsh` を `dot_config/zsh/rc/` 配下へ移動、sheldon の glob を `{options,secrets,for_development}.zsh` に拡張

### Fixed — Low
- **L-1**: 危険 alias `gu='git add . && git commit && git push'` を削除（`add .` で意図しないファイルを巻き込む）
- **L-2**: `aliases.zsh` の `$HOME` ハードコード除去。冗長な `claude` alias 削除、bun の `/home/kiyama` 直書きを `$HOME/.bun/_bun` + `[[ -r ... ]]` ガードに変更
- **L-3**: `alias navit= navi --tldr`（`=` 後の空白）を `alias navit='navi --tldr'` に修正
- **L-4**: `pandapdf.zsh` から PII（学籍番号風 ID + 姓）を除去。出力ファイル名を `PANDAPDF_OUTPUT` 環境変数 / 第2引数で上書き可能に変更
- **L-5**: rustup インストール行に `sh -s -- --no-modify-path -y` を追加（curl-pipe-sh の二重防御 + run_once 非対話実行）
- **L-6**: `HISTSIZE=200` を `HISTSIZE=10000` に変更（`SAVEHIST=100000` と整合）
- **L-7**: `aliases.zsh` と `integrations/{lsd,bat,fd,ripgrep}.zsh` の `ls / cat / find / grep` 重複に「coreutils フォールバック / integrations 側で上書き」のコメントを追記して見通しを改善
- **L-8**: `dot_config/chezmoi/.password_manager.sh` と `dot_local/share/chezmoi/dot_password_manager.sh` の dead copy を削除（アクティブはルートの `.executable_password_manager.sh` のみ）
- **L-9**: `dot_config/zsh/completions/` を削除（`my_fpaths` は `dot_zfunc/` のみ参照、dead code）

### Reverted / 誤判断
- **M-1**: `empty_shorthands.toml` を `shorthands.toml` にリネームしたが、chezmoi の `empty_` は「空ファイルでもデプロイする」メタデータ prefix で、target 名は strip 後の `shorthands.toml` で正しかった。差し戻し済（学び: `empty_` / `private_` / `executable_` / `readonly_` は target 名から strip される、削除しない）

### Follow-ups completed
- **F-1**: `upd` を function 化し `topgrade -y ... && chezmoi_apply` 連結に変更。option A 採用で zsh 関数のまま完結（commit `ee5108c`）
- **F-2**: `CLAUDE.md` の Bitwarden セッション節を `bw_session` / `bw_lock` ヘルパ前提に書き換え、`README.md` の update 節を `bw_session` → `chezmoi apply` → `bw_lock` フローに更新

---

## 2026-04-29 以前

それ以前の変更は `git log` を参照。主要マイルストーン:
- 2026-04-29: `chore: pin sheldon plugins, harden mise/zsh config, clean up dotfiles` (`7cd0cb0`)
- 2026-04-29: `refactor: harden zsh/mise/sheldon config and fix review items` (`39ec75a`)
- 2026-04-29: `docs: rewrite CLAUDE.md and add Contents section to README` (`3a5278b`)
- それ以前は ad-hoc な commit message（`Update`, `Upd` 等）が多く、CHANGELOG として整理されていない
