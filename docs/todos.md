# Open TODOs

最終更新: 2026-04-29
完了済みタスクは [`CHANGELOG.md`](../CHANGELOG.md) を参照。
当初のレビューは `7cd0cb0` / `39ec75a` / `4424716` / `ee5108c` 周辺のコミットで C-1 〜 L-9 / F-1 / F-2 をすべて消化済み。本ファイルは派生フォローアップ + 新規タスクの追跡用。

---

## アクティブタスク

### F-3. wired 通知の左クリック / 右クリックアクション実装 🆕
- 背景: B 案で Claude Code → wired のデスクトップ通知が復活し、`--expire-time=0` で自動消去されなくなった (`dot_local/bin/executable_claude-notify-sound.sh`)。次のステップとして popup を **左クリックで発信元セッションへフォーカス** / **右クリックでコンテキストメニュー (or 即時 dismiss)** にしたい。
- 該当: `dot_local/bin/executable_claude-notify-sound.sh` 拡張、または新規 `dot_local/bin/executable_claude-notify-dispatch.sh` (仮)
- 設計メモ:
  - libnotify のアクション: `notify-send --action="default=Open" --wait` で popup にクリック領域を生やせる。`--wait` は ActionInvoked シグナルが返るまでブロックし、action key を stdout に出力。
  - Claude Code hook 本体は短命で `--wait` を同期保持できない → **バックグラウンドサブシェル** で `notify-send --wait` を fork し `setsid` でセッション分離。hook 自体は即 return。
  - 左クリック (`default` アクション):
    - hook payload の `session_id` / `transcript_path` / `cwd` を `--hint=string:x-claude-session:<id>` 等に載せて受け渡す
    - tmux 上で動いていれば `tmux switch-client -t <session>` + `select-pane -t <pane>` で前面化
    - 既存 `dot_config/tmux/scripts/tmux-claude-*.sh` (commit `4c00b7a` / `9b12f16` で導入) のロジックを流用可能か確認
    - tmux 外なら fallback として `cwd` を持つ ghostty / kitty ウィンドウを `wmctrl` / `swaymsg` で focus、もしくは `transcript_path` を `$EDITOR` で開く
  - 右クリック: wired のデフォルト (dismiss) を尊重するか、`--action=open-transcript=Open transcript` 等で別アクションを増やすか検討。`wired.ron` の `notification.action_default` / `action_anywhere` の挙動を実機で確認してから決める。
  - 多重発火対策: 同一 session に対する未処理 popup が複数残っているとアクション競合する → libnotify の `replace-id` で前の popup を上書きするか、session_id をキーに de-dup。
- 対応:
  - [ ] hook payload (`session_id`, `transcript_path`, `cwd`) を `--hint=string:x-claude-session:<value>` で `notify-send` に乗せる
  - [ ] `notify-send --wait --action=default=Focus` を fork するディスパッチャースクリプトを追加 (`setsid` + `disown` で hook 親プロセスから分離)
  - [ ] アクション受信時に tmux セッション特定 → switch-client。tmux に無ければ `transcript_path` を `$EDITOR` で開く fallback
  - [ ] 右クリックの挙動を決定 (デフォルト dismiss を維持 / カスタムアクション追加)
  - [ ] wired 側で `default` アクションが左クリックで発火することを `wired.ron` 既定構成のまま動作確認 (必要なら `notification.action_default = "left_click"` の設定例をドキュメント化のみ。wired.ron 自体は変更しない)
  - [ ] `replace-id` ベースで同一 `session_id` の通知が積み上がらないようにする (任意)
- 注意:
  - `notify-send --wait` は呼び出した libnotify プロセスが生きている間しかアクションを受けない。fork 後に親 hook が exit すると D-Bus name が消える可能性 → `setsid` でセッションリーダー化、または常駐 helper デーモン化を検討。
  - tmux 外で起動された Claude Code (素の terminal) のフォーカス手段は WM 依存 (Wayland: `swaymsg`, X11: `wmctrl` / `xdotool`)。最小は tmux 連携のみ実装し、それ以外は no-op + ログにとどめる。

### F-4. wired-notify を chezmoi run_once で管理対象化 🆕
- 背景: 通知用 daemon `wired-notify` のバイナリが何らかの理由で OS から消えていた (`/usr/bin/wired` 不在 → `wired.service` が `status=203/EXEC` で 140+ 回 restart loop)。設定 (`dot_config/wired/wired.ron`) と systemd unit (`dot_config/systemd/user/wired.service`) は chezmoi 管理下にあるが、**パッケージ本体は `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl` の paru パッケージ列に含まれていない** ため、新規マシンや AUR クリーンアップ後に同じ事態が再発する。今回 (2026-04-29) は `paru -S wired-notify` で手動復旧済み。
- 該当: `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl` (line 65-81 付近の `PACKAGES` ヒアドキュメント)
- 対応:
  - [ ] `PACKAGES` に `wired-notify` を追加 (Manjaro/Arch では AUR; `paru -S` で取得)
  - [ ] 追加後にクリーンな環境で `chezmoi init --apply kkiyama117` を試して再現性を確認 (任意 / 過剰なら省略)
  - [ ] `README.md` 「初期セットアップで入るパッケージ」相当の節があれば併せて更新
- 注意:
  - `wired-notify` は AUR パッケージのため `paru` 経由必須 (`pacman -S` 単独では入らない)。
  - 再インストール後は `systemctl --user start wired.service` を一度叩く必要があるかもしれない (今回は手動 start で復活)。

---

## デファード（着手判断保留・小粒なフォローアップ）

過去レビューから派生したが、現状運用で痛みが小さいため後回しにしているもの。

- **API キー lazy export 化**（C-1 派生）: 利用直前にだけ `OPENROUTER_API_KEY` を export するラッパー関数 / direnv 移行の検討。現状 `private_secrets.zsh.tmpl`（0600）で許容範囲
- **fast-syntax-highlighting の本家移行**（H-3 / H-12 派生）: `auscompgeek/fast-syntax-highlighting` のメンテ状況を再確認し、`zdharma-continuum/fast-syntax-highlighting` への乗り換えを検討
- **mise `trusted_config_paths` の縮小**（H-4 派生）: `~/programs` 全体ではなくプロジェクト単位に絞る。TODO コメントのみ残置済
- **`dircolor` alias の整理**（H-10 派生）: `aliases.zsh:7` の手動再読込用 alias は残置中。整理は別タスクで
- **chezmoi ソース dir 内の自己参照整理**（L-8 派生）: `dot_local/share/chezmoi/dot_keep`
- **`bw_lock` 自動化**（F-2 派生）: `precmd` フックで一定時間アイドルなら自動 `bw_lock`。過剰になりやすいため現時点では明示運用に留める

---

## sheldon を脇に置く判断材料（参考メモ）

現状 sheldon が担っている責務:
- 外部 zsh プラグインの取得・defer ロード
- `mise activate` / `starship init` / `compinit` / `colors` の inline 実行
- ローカルファイル（`rc/{options,aliases,completion,functions/*,my_plugins/*}.zsh`, `integrations/*.zsh`）の取り込み
- PATH 追加（`~/.config/zsh/bin`）
- fpath 追加（`~/.config/zsh/.zfunc`）

移行候補:
- **A. zinit / antidote へ移行**: lockfile + pin 標準化（H-3 が同時解決）。投資対効果が最も高い
- **B. sheldon を残し pin 運用**: `[plugins.*]` に `rev` か `tag` を必須化するだけ。最小コスト（**現状はこれを採用**）
- **C. 完全撤廃**: `.zshrc` 直書き + git submodule。依存ゼロだが defer / キャッシュ自前

候補 B → A の段階移行が現実的。
