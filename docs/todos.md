# Open TODOs

最終更新: 2026-04-30 (F-4 を nix 移行方針に変更)
完了済みタスクは [`CHANGELOG.md`](../CHANGELOG.md) を参照。
当初のレビューは `7cd0cb0` / `39ec75a` / `4424716` / `ee5108c` 周辺のコミットで C-1 〜 L-9 / F-1 / F-2 をすべて消化済み。本ファイルは派生フォローアップ + 新規タスクの追跡用。

---

## アクティブタスク

### F-3. wired 通知の左クリック / 右クリックアクション実装 (v1 完了 / follow-up あり)
- 背景: B 案で Claude Code → wired のデスクトップ通知が復活し、`--expire-time=0` で自動消去されなくなった (`dot_local/bin/executable_claude-notify-sound.sh`)。次のステップとして popup を **左クリックで発信元セッションへフォーカス** / **右クリックで個別 close**。設計ドキュメントは [`superpowers/specs/2026-04-30-wired-click-actions-design.md`](superpowers/specs/2026-04-30-wired-click-actions-design.md)、実装計画は [`superpowers/plans/2026-04-30-wired-click-actions.md`](superpowers/plans/2026-04-30-wired-click-actions.md) を参照。

#### v1 (実装済み, 2026-04-30)
- [x] hook を `claude-notify-hook.sh` (orchestrator) / `claude-notify-sound.sh` (sound worker) / `claude-notify-dispatch.sh` (popup + action loop) の 3 ファイルに分割
- [x] hook payload (`session_id`, `message`) と env (`TMUX_PANE`, tmux session 名) を環境変数で dispatcher に受け渡し
- [x] `notify-send --print-id --wait --action=default=Focus` で popup を保持し、ActionInvoked 受領時に tmux focus + `gdbus CloseNotification` で auto-dismiss
- [x] `wired.ron` shortcuts を **`notification_action1: 1` (左)** / `notification_close: 3` (右) / `notification_closeall: 2` (中) に組み替え (v1.1 ホットフィックス: `notification_interact` は本文の URL を開こうとするだけで `ActionInvoked` を発火しないため、`notification_action1` に差し替え)
- [x] dispatcher を `setsid` で hook 親 (claude) から分離し、hook は即 exit 0
- [x] `docs/manage_claude.md` §5.7 と `docs/claude_tmux_cheatsheet.md` §5 にクリックアクション表を追記

#### F-3.next (follow-up, 未着手)
- [ ] 同一 `session_id` の通知が積み重なった場合の **`replace-id` ベース de-dup**。state ファイル or libnotify hint で session→notif_id を覚えて上書き
- [ ] **bare terminal fallback** — tmux 外で起動された Claude セッションを左クリックした時に `wmctrl` (X11) / `swaymsg` (Wayland) で cwd を持つ window を focus、または `transcript_path` を `$EDITOR` で開く
- [ ] **セッション消失時の自動再オープン** — kill されたあと残った popup を左クリック → `tmux_claude_new` 相当のロジックで tmux session を再生成 + `claude --resume <session_id>` で claude を復元
- [ ] **右クリックの拡張アクション** — 単純 close 以外に「transcript を開く」など二重アクションを検討 (要 wired/notify-send の追加 action 設計)
- [ ] **dispatcher を 1 本の常駐 helper daemon に集約** — D-Bus signal を直接 listen する案 (ブレストの案 B)。多重 popup 時の状態管理が綺麗になる代わりに systemd unit が増える

### S-1. シェルコマンド発見系3層 (zsh補完 / tldr / navi) の役割整理と統合 🆕
- 背景: 現在 zsh autocomplete + tealdeer + navi の3つを併用しているが、明示的な役割分担とキー動線、chezmoi での管理粒度が未整理。冗長なストック (例: navi に tldr 相当を貯める) と新規マシンでの再現性低下を防ぐため、A→B→C の段階で進める。フェーズ A は本コミット前後で着手、B/C は後続。
- 該当範囲:
  - `dot_config/zsh/` 配下のキーバインド / widget 定義 (`rc/functions/`, `rc/aliases.zsh` 等)
  - `dot_config/navi/` (新規予定: cheats と config)
  - `dot_config/tealdeer/` (新規予定: config)
  - `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl` の `PACKAGES` (パッケージ追加対象)
  - `docs/` 配下のポリシードキュメント (例: `docs/shell_discovery.md` 仮)

- フェーズ A (まず実施 = ロール分離のドキュメント化): **完了 (2026-04-30)** → [`shell_discovery.md`](shell_discovery.md)
  - [x] `docs/shell_discovery.md` を新規作成し、3層の使い分けを明文化
  - [x] zsh 補完 = 既に打ち始めたコマンドの続きをタブで補完 (無意識動線)
  - [x] tldr (tealdeer) = コマンド単位の公式チートシート参照 ("どう使うんだっけ" レベル)
  - [x] navi = 自分用 / プロジェクト固有のワンライナー保存・引数埋め込み実行 (個人スニペット集)
  - [x] 「navi に tldr 相当 (公式コマンドの基本用法) を蓄積しない」運用ルールを明記
  - [x] `docs/keybinds.md` への参照を貼り、後続フェーズでキーバインド統一案を反映できる余地を残す

- フェーズ B (大半完了, 2026-04-30):
  - [x] `dot_config/tealdeer/config.toml` を chezmoi 管理下に追加 (`chezmoi add` で取り込み済み)
  - [x] tealdeer の cache 更新方式: **`config.toml` 内の `auto_update = true` + `auto_update_interval_hours = 24`** で自然に週次未満の頻度で更新されるため、`run_onchange` / cron / pueue は採用しない判断
  - [ ] `dot_config/navi/` にチートシート (`*.cheat`) を配置 (config.yaml は既に管理下、cheat 棚卸しはフェーズ C 寄りなので保留)
  - [x] `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl` の `PACKAGES` に `tealdeer` / `navi` を追加 (実機ファイル確認: lines 73, 76 で既に追加済み。todos.md だけが古かった)
  - [x] zsh 側の widget / alias を `rc/integrations/{navi,tealdeer}.zsh` に整理し、Sheldon `[plugins.software_integrations]` 経由で `zsh-defer` ロード
  - [x] tldr / navi の起動キーを実機検証して確定:
    - tldr: `Alt+H` は zsh `run-help` で使用中のため割り当てなし。コマンド名 `tldr` / `tld` / `tldrf` (skim/fzf 連携) で起動
    - navi: 既定の `Ctrl+G` を採用 (zsh `send-break` を上書き)。SKK Hiragana / 変換中は SKK が `^G` を `abort` で消費するため、Latin モード時のみ widget 発動。fallback として `nv` / `navi` / `navit` alias を提供
    - 詳細: [`shell_discovery.md`](shell_discovery.md) §4 / [`keybinds.md`](keybinds.md) §3.4 末尾

- フェーズ C (後で = navi cheat 棚卸し):
  - [ ] 既存 `dot_config/zsh/rc/aliases.zsh` 等の自作 alias / 関数のうち、引数を取りパラメタライズ可能なものを抽出
  - [ ] `*.cheat` 形式に書き換えて navi 管理へ移行
  - [ ] 移行済み alias は削除 or thin wrapper のみ残し、シェル起動コストを下げる
  - [ ] チートシートのタグ規約を決定 (例: `@chezmoi`, `@bitwarden`, `@tmux`, `@git`, `@claude`)

- 注意:
  - 全 alias を navi に移すのは過剰 — 1〜2 単語で頻打する alias はキータイプ速度の利得があるため残置基準を作る (例: 引数を3つ以上取る or 30 文字以上のものだけ navi 化)
  - tealdeer の初回 cache fetch はネットワーク必須 → 新規マシン bootstrap で `run_onchange` が走るタイミングと bw_session unlock のタイミング順序に注意
  - navi の widget が zsh の line editor フックに割り込むため、`fzf-tab` / `zsh-autosuggestions` / `zsh-vi-mode` 等の widget と衝突する可能性。フェーズ B で `bindkey -L` 出力を取って差分管理する

### F-4. wired-notify を nix で管理対象化 🆕 (方針更新: 2026-04-30)
- 背景: 通知用 daemon `wired-notify` のバイナリが何らかの理由で OS から消えていた (`/usr/bin/wired` 不在 → `wired.service` が `status=203/EXEC` で 140+ 回 restart loop)。設定 (`dot_config/wired/wired.ron`) と systemd unit (`dot_config/systemd/user/wired.service`) は chezmoi 管理下にあるが、**パッケージ本体は宣言的に管理されていない**ため、新規マシンや AUR クリーンアップ後に同じ事態が再発する。今回 (2026-04-29) は `paru -S wired-notify` で手動復旧済み。
- 方針: `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl` の `paru` ヒアドキュメントへ追加するのではなく、**今後は `nix` (Home Manager 等) でパッケージ/プログラム全体を宣言的に管理する方向**へ移行する。F-4 は nix 移行の一部として処理する。
- 該当 (移行先):
  - 新設予定の nix 設定 (例: `nix/` ディレクトリ または Home Manager の `home.packages`)
  - 既存 `.chezmoiscripts/run_once_all_os.sh.cmd.tmpl` の `PACKAGES` は段階的に nix へ巻き取り
- 対応:
  - [ ] nix (flakes + Home Manager 想定) のルート設定を chezmoi 管理下に追加
  - [ ] `wired-notify` を nix の packages に登録 (nixpkgs に存在しなければ overlay / flake input で対応)
  - [ ] `dot_config/systemd/user/wired.service` の `ExecStart` パスを nix profile 経由 (`~/.nix-profile/bin/wired` 等) に揃えるか確認
  - [ ] 既存 `paru` PACKAGES のうち nix で代替可能なものをリストアップし、移行範囲を決める (一気に全移行は過剰)
  - [ ] クリーンな環境で `chezmoi init --apply kkiyama117` → nix 適用 の流れを試して再現性を確認
  - [ ] `README.md` / `CLAUDE.md` の「Bootstrap (新規マシン)」節を nix ベースの手順に更新
- 注意:
  - Manjaro 上で nix を使う場合、`systemd-nix` 起動順や `XDG_DATA_DIRS` への nix profile の追加が必要 (.desktop / fonts 等を nix から拾わせる場合)。
  - `wired-notify` のバージョン差で `ron` 設定の互換性が崩れる可能性 — nix で pin したバージョンと現行 `dot_config/wired/wired.ron` の妥当性を移行時に確認する。
  - paru / nix の二重管理になる過渡期は、どのパッケージがどちら経由で入っているかを明示するメモを `docs/` に置く (例: `docs/package_management.md`)。

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
