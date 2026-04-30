# Open TODOs

最終更新: 2026-05-01 (F-7 branch-out worktree spawner 実装)
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

#### F-3.next (follow-up)
- [x] 同一 `session_id` の通知が積み重なった場合の **`replace-id` ベース de-dup** (実装済み, 2026-04-30)
  - state file: `${XDG_RUNTIME_DIR:-/tmp}/claude-notify/sessions/<sid>.id` (1 行 = 直近の notif_id)
  - dispatcher 起動時に prev_id を読んで `notify-send --replace-id` を渡し、wired 側で同 popup を in-place 更新
  - session_id が空なら旧挙動 (replace-id 渡さず新規 popup)
  - state file の TTL / セッション終了時 cleanup は別 follow-up (古い id の replace は wired が黙って無視するので無害)
  - smoke test: 同 session で 3 連発 → popup 1 個に集約 / 別 session 同時発火 → 2 個独立
- [ ] **bare terminal fallback** (部分実装済み: `xdotool` (X11) / `swaymsg` (Wayland) で kitty/ghostty/wezterm/Alacritty を window-class で focus する処理は dispatch.sh に実装済み 〜L88)
  - 残: cwd ベースで「該当 Claude プロセスが居る window だけ」を選んで focus、または `transcript_path` を `$EDITOR` で開くアクションを追加
- [ ] **セッション消失時の自動再オープン** — kill されたあと残った popup を左クリック → `tmux_claude_new` 相当のロジックで tmux session を再生成 + `claude --resume <session_id>` で claude を復元
- [ ] **右クリックの拡張アクション** — 単純 close 以外に「transcript を開く」など二重アクションを検討 (要 wired/notify-send の追加 action 設計)
- [ ] **dispatcher を 1 本の常駐 helper daemon に集約** — D-Bus signal を直接 listen する案 (ブレストの案 B)。多重 popup 時の状態管理が綺麗になる代わりに systemd unit が増える
- [x] **state file cleanup** (実装済み, 2026-04-30) — F-3.next #1 の副作用対応として `dot_local/bin/executable_claude-notify-cleanup.sh` + `dot_config/systemd/user/claude-notify-cleanup.{service,timer}` を追加。`OnCalendar=daily Persistent=true RandomizedDelaySec=15min` で `*.id` を mtime ベース TTL (既定 7 日、`CLAUDE_NOTIFY_CLEANUP_TTL_DAYS` で上書き可) で剪定し、`.tmp.XXXXXX` 残骸 (mktemp 中断時の漏れ) を 60 分超えで併せて削除。base_dir suffix チェックで env 注入耐性、`ProtectSystem=strict` + `ReadWritePaths=%t/claude-notify` で書き換え範囲を runtime dir に限定。`atime` は `noatime` / `relatime` mount で信頼できないため mtime に切替 (dispatcher が atomic `mv -f` で更新する都度 mtime が引かれるので "最後の通知発火時刻" として機能)。bootstrap は `.chezmoiscripts/run_onchange_after_enable-claude-notify-cleanup.sh.tmpl` 経由で `systemctl --user daemon-reload` + `enable --now` を idempotent に実行 (unit / helper の sha256 を埋めて変更検知)。smoke (合成 state dir): stale 2 件 + 古い `.tmp` 1 件のみ削除、fresh 4 件は保持、unexpected base 引数は exit 0 ガード発動を実機 PASS

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

- フェーズ C (棚卸し完了, 2026-04-30):
  - [x] 既存 `dot_config/zsh/rc/aliases.zsh` / `rc/functions/` / `rc/my_plugins/` を §3-2 基準で精査 → **移行候補ゼロ** (詳細は [`shell_discovery.md`](shell_discovery.md) §6-1)
  - [x] `*.cheat` 形式に書き換えて navi 管理へ移行 → 移行候補が無いため対象なし。代わりに **新規スニペット用 seed として** `dot_config/navi/cheats/{chezmoi,git}.cheat` を 2 ファイル配置
  - [x] 移行済み alias の削除 / thin wrapper 化 → 移行案件無しのためスキップ
  - [x] チートシートのタグ規約を決定 → `chezmoi` / `bitwarden` / `tmux` / `git` / `claude` / `chezmoi-private` を採用 ([`shell_discovery.md`](shell_discovery.md) §6-2)

- 注意:
  - 全 alias を navi に移すのは過剰 — 1〜2 単語で頻打する alias はキータイプ速度の利得があるため残置基準を作る (例: 引数を3つ以上取る or 30 文字以上のものだけ navi 化)
  - tealdeer の初回 cache fetch はネットワーク必須 → 新規マシン bootstrap で `run_onchange` が走るタイミングと bw_session unlock のタイミング順序に注意
  - navi の widget が zsh の line editor フックに割り込むため、`fzf-tab` / `zsh-autosuggestions` / `zsh-vi-mode` 等の widget と衝突する可能性。フェーズ B で `bindkey -L` 出力を取って差分管理する

### S-2. navi config.yaml を新版 navi 互換に修正 (完了, 2026-04-30)
- 背景: S-1 phase C 作業中に、現行 `dot_config/navi/config.yaml` (2024 年版) が手元の navi 2.24.0 で `Error parsing config file: finder: Failed to deserialize finder: sk at line 17 column 3` を出して finder 設定が読み込めないことが判明。
- 該当: `dot_config/navi/config.yaml` → `dot_config/navi/config.yaml.tmpl` へ rename + 新書式に書き換え
- 対応:
  - [x] navi 2.24.0 を確認し、`navi --help` の許容値 `[possible values: fzf, skim]` と `info config-example` の出力で新 schema を特定
  - [x] `finder.command: sk` を `skim` に修正 (deserialize エラー解消、rendered config を `NAVI_CONFIG` 経由で食わせて `Error parsing config file` が出ないことを確認)
  - [x] `cheats.path` (DEPRECATED) を `cheats.paths` (リスト) に書き替え。chezmoi template `{{ .chezmoi.homeDir }}/.config/navi/cheats` で絶対パスを埋め込む方針に変更 (navi の tilde / `$HOME` 展開は不安定なため)
  - [x] `client.tealdeer: true` を追加し `navi --tldr` (= `navit` alias) を tealdeer 連携に固定
  - [x] `dot_config/zsh/rc/integrations/navi.zsh` のコメントを S-2 完了状態に更新
- 残: 実機で `chezmoi apply` → `~/.config/navi/config.yaml` 反映後に Ctrl+G widget で chezmoi/git cheat が拾えるかをユーザ操作で確認 (auto mode の非 tty 環境では検証困難なため)
- 注意:
  - 環境変数 `NAVI_PATH` は navi 2.24 で参照されないため使わない (誤導を避けるため `navi.zsh` でも export しない)。
  - `navi info cheats-path` / `info config-path` は config を反映表示しない仕様 (常にデフォルトを返す) のため、反映確認には `info` 系を使わず実際の cheat ロード動作で判断する。

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

### F-5. Claude cockpit state tracking 残課題 🆕 (v1 完了 / follow-up あり)
- 背景: 2026-04-30 に [`superpowers/plans/2026-04-30-claude-cockpit-state-tracking.md`](superpowers/plans/2026-04-30-claude-cockpit-state-tracking.md) を Subagent-Driven で完走 (15 commits, `8c0526e`〜`eec2fe9`)。Claude hook → atomic state file → tmux summary / 階層 fzf switcher / next-ready jump の pipeline が live で稼働中 (summary.sh が `⚡ N ⏸ M ✓ K ` を実出力している)。Final review で HIGH 1 件 + MEDIUM 2 件は別 commit で修正済み。残りは spec の LOW 2 件と、auto mode 非 tty 環境では検証できなかった 8-step manual smoke。
- 該当範囲:
  - `dot_local/bin/executable_claude-cockpit-state.sh` (LOW-1 logger 追加)
  - `dot_config/tmux/scripts/cockpit/executable_next-ready.sh` (LOW-2 display-message duration 明示化)
  - `docs/manage_claude.md` 末尾の "Cockpit State Tracking — Smoke Tests" 節 (manual 検証手順)

#### v1 (実装済み, 2026-04-30)
- [x] `claude-cockpit-state.sh` hook entry (UserPromptSubmit/PreToolUse → working / Notification → waiting / Stop → done) を atomic write (tmp + mv) で実装
- [x] `dot_config/claude/settings.json` の `hooks` 配列 4 つに append (既存 `observe.sh` / `claude-notify-hook.sh` は保持)
- [x] `cockpit/summary.sh` で status-right を `⚡ N ⏸ M ✓ K ` 形式に置換 (5 秒間隔の pgrep 廃止)
- [x] `cockpit/switcher.sh` (session/window/pane 階層 fzf, Enter=switch / Ctrl-X=kill / Ctrl-R=reload, claude-* 限定で worktree-aware kill)
- [x] `cockpit/next-ready.sh` (inbox 順 = session asc / window idx asc / pane idx asc で done pane 循環ジャンプ)
- [x] `cockpit/prune.sh` (orphan cache 掃除) を tmux server-start に `run -b` で組み込み + switcher 起動時にも実行
- [x] `claude-kill-session.sh` 末尾に cache 削除を追記 + optional `$1` で session-name 引数を受け、switcher Ctrl-X が選択 session を正しく kill するよう修正 (final-review HIGH `6365159`)
- [x] `claude_table.s` を `cockpit/switcher.sh` に再配線、新規 `claude_table.N` で next-ready ジャンプ
- [x] 旧 `claude-status-count.sh` / `claude-pick-session.sh` を削除
- [x] `docs/manage_claude.md` §5.2 / §5.3 / §5.5 + `docs/keybinds.md` §2.2 を新スクリプト群に合わせて更新 (final-review MEDIUM `eec2fe9`)
- [x] `docs/manage_claude.md` 末尾に 8-step smoke test 節を追加

#### F-5.next (follow-up, 未着手)
- [ ] **8-step manual smoke の実機通し** — 実 Claude session を 2 つ立てて UI の `⚡ / ⏸ / ✓` 遷移と `prefix + C → s/N/k` を体感確認 ([`docs/manage_claude.md`](manage_claude.md) 末尾節)。auto mode の非 tty 環境では UI 確認不能だったので残置。失敗時は spec §9 の合格条件と差分を本ファイルに追記
- [x] **LOW-1: spec §8 の `logger -t claude-cockpit-state` 呼び出しを実装** (実装済み, 2026-04-30) — `claude-cockpit-state.sh` の `mkdir` 失敗 / atomic `mv` 失敗 / tmp write 失敗の 3 経路に `command -v logger >/dev/null 2>&1 && logger -t claude-cockpit-state "<reason>: <path>"` を仕込み済み。`exit 0` の絶対契約は不変
- [x] **LOW-2: `next-ready.sh` の display-message に `-d 1000` を明示** (実装済み, 2026-04-30) — cache_dir 不在時 / done pane なしの 2 経路を `tmux display-message -d 1000 "no ready claude pane"` に変更。spec §6.4 文面と一致
- [ ] **次回 `tmux kill-server` 後の status-right 確定** — 今回 tmux-continuum の restore 干渉で一度 stale 状態になり `chezmoi apply --force` で修復。次回 tmux 再起動時に status.conf 新版が確実に反映されることを確認 (このフォローアップは observation のみで PR 不要)。**2026-04-30: `@continuum-restore` を無効化したので auto-restore 干渉そのものは消えた前提での再観察**
- [ ] **`tmux kill-server` 経由の prune 統合テスト** — server 再起動時の `run -b '~/.config/tmux/scripts/cockpit/prune.sh'` が orphan cache を実際に消すかを実環境で 1 度確認
- [x] **`@continuum-restore` 無効化** (2026-04-30) — `dot_config/tmux/conf/plugins.conf:15` をコメントアウト。default session と復元 session が共存して window が混在する事故を回避。復元は `prefix + Ctrl-R` (tmux-resurrect) で手動。`docs/manage_claude.md` §5.6 の TPM プラグイン表も更新済み

- 注意:
  - hook ordering: cockpit-state.sh は **既存 observer の後ろに append** してある (spec §Notes 通り)。state hook は数 ms で完了するため observer / notify-hook の latency に影響しない
  - cache layout (`panes/<S>_<P>.status`) は上流 `tmux-agent-status` と同形式 — 将来上流に乗り換える場合は `~/.cache/claude-cockpit/panes` → `~/.cache/tmux-agent-status/panes` の symlink で状態を継承可能
  - SIGKILL で Claude が強制停止された場合 Stop hook が発火しないため `working` のまま残るが、prune.sh が tmux 側で消えた pane の cache を回収する設計

### F-6. tmux session/window 階層再構成 (完了 2026-04-30)
- 背景: flat な `claude-<branch>` session スキームを **session = main repo basename / window = branch / 各 window 2 pane** に再構成。複数 repo で同名 branch (`develop` 等) を持つ際の session 名衝突を解消し、cockpit 階層 fzf スイッチャの 3 段表現を活かす。設計は [`superpowers/specs/2026-04-30-tmux-repo-session-window-design.md`](superpowers/specs/2026-04-30-tmux-repo-session-window-design.md)、実装計画は [`superpowers/plans/2026-04-30-tmux-repo-session-window.md`](superpowers/plans/2026-04-30-tmux-repo-session-window.md)。
- 該当: `tmux-claude-new.sh` / `claude-kill-session.sh` / `bindings.conf` / `manage_claude.md` / `keybinds.md`
- 対応:
  - [x] Task 1: tmux-claude-new.sh を repo-session + branch-window scheme に refactor
  - [x] Task 2: claude-kill-session.sh を window-scoped kill に縮小、`@claude-managed` 判定を導入
  - [x] Task 3: bindings.conf の `claude_table.k` の note と confirm message を更新
  - [x] Task 4: docs/manage_claude.md と docs/keybinds.md を新スキーマに更新
  - [x] Task 5: 8-step 手動スモークの実機通し (2026-04-30 PASS 19 / FAIL 0)
- 注意:
  - 既存 `claude-*` session には介入しない (自然消滅させる migration)
  - tmux-continuum の resurrect で旧 session 名が部分復活する可能性あり (要 follow-up)
  - 異 repo + 同 basename の collision は v1 では非対応 (spec §3.4 / Q1)

### F-7. /branch-out worktree spawner (実装済み 2026-05-01)
- 背景: ユーザの依頼を受けて現セッションの Claude が**深く考えず** branch 名のみ生成し、新規 git worktree + tmux window + fresh Claude セッションへハンドオフして "深く考えるのは新セッション" と役割分担する spawn 動線。F-6 の repo-session + branch-window scheme と整合。参考にした命名規則: Qiita (syukan3) の `worktrees/{type}-<name>` prefix 案、ChristopherA gist の bare-repo + worktree 並置案 (今回は bare 化までは踏み込まず命名のみ採用)。
- 配置と命名:
  - worktree path: `${XDG_DATA_HOME:-$HOME/.local/share}/worktrees/<repo>/<sanitized-branch>` (XDG 中央集約)
  - branch 名: `<type>/<kebab-summary>` (`feat`/`fix`/`refactor`/`docs`/`chore`)
  - tmux window 名: branch 名そのまま (F-6 命名と整合、`/` は sanitize で `-` 置換)
- 実装:
  - [x] `dot_config/tmux/scripts/executable_tmux-claude-new.sh` に `--worktree-base <dir>` / `--prompt <text>` の 2 flag 追加 (既存 sibling 配置 `${main_repo}-${safe}` は flag 未指定時のデフォルトとして温存。`mkdir -p` で worktree 親を idempotent 作成、`printf %q` で prompt を bash-quote → tmux send-keys → 対話シェル)
  - [x] `dot_config/claude/commands/branch-out.md` を新規作成。slash command 本体は (a) `$ARGUMENTS` から `<type>/<kebab>` を1秒で導出 → (b) `tmux-claude-new.sh '<branch>' --worktree-base ... --prompt '<msg>'` を 1 回叩く → (c) 1 行報告。**現セッションの Claude が依頼内容を分析・実装することを明示的に禁止**
  - [x] `dot_config/git/config.tmpl` に `[alias] wt = worktree / wtl = worktree list / wta = worktree add -b` を追加 (Qiita 案準拠、手動 `git wt` 操作の利便)
- スモーク:
  - bash -n / `-h` 出力 / path 計算ユニットテスト (`/home/kiyama/.local/share/worktrees/chezmoi/feat-branch-out`) / `printf %q` で日本語・空白・metachars 通過確認 PASS
  - `chezmoi managed` で 3 ファイル (`branch-out.md` / `git/config` / `tmux-claude-new.sh`) が tracked であることを確認
  - 実機 e2e (実際に `/branch-out` を叩いて新 window へフォーカス) は tty 必要なため未実施 — 次回対話時に手動確認
- 残課題 / follow-up:
  - [ ] 既存 sibling 配置 (`${main_repo}-${branch}`) と新規 centralized 配置の **混在運用ルール** をドキュメント化 (F-6 spec §3 への追記候補)
  - [ ] `claude-pick-branch.sh` (prefix+C n 経由) も `--worktree-base` を受け取れるよう拡張するか、cockpit popup 内で centralized デフォルト化するか判断
  - [ ] worktree の **掃除** (merged branch の worktree 自動 prune) helper — 現状 `git wt remove` 手動。F-3.next #5 のような systemd timer でやるか手動運用かは未決
  - [ ] `--prompt` の長文 (~数 KB) における send-keys 遅延 / quoting 限界の実測 — 通常用途では問題なし想定

### F-8. cockpit 状態 file の死蔵対策 🆕 (v1 完了 / 残: eBPF 検討)
- 背景: claude が `/exit` で終わった、あるいは SIGKILL / OOM / pane クローズで terminal 内 claude プロセスが終了した場合、`${XDG_CACHE_HOME}/claude-cockpit/panes/<S>_<P>.status` が **最後の hook 値で残り続けるバグ**。`cockpit/summary.sh` の `⚡ N ⏸ M ✓ K ` カウントが減らず、`cockpit/next-ready.sh` (prefix+C+N) も幽霊 pane を ready 候補として選んでしまう。F-5 cockpit と F-6 repo-session の組合せで顕在化した。
- 該当: `dot_local/bin/executable_claude-cockpit-state.sh` / `dot_config/claude/settings.json` / `dot_config/tmux/scripts/cockpit/{summary,next-ready,switcher,prune}.sh` / `docs/superpowers/specs/2026-04-30-claude-cockpit-state-tracking-design.md`
- 対応 v1 (実装済み):
  - [x] **A: graceful exit パス** — Claude Code の `SessionEnd` hook (`/exit` / `/clear` / `/logout` 等) を `claude-cockpit-state.sh` に追加し、status file を `rm -f` で削除。`settings.json` にも `SessionEnd` entry を追加
  - [x] **B: defensive reader-side filter** — `summary.sh` / `next-ready.sh` / `switcher.sh` に `pane_current_command == claude` ガードを追加。SessionEnd が発火しなかった場合（SIGKILL / OOM / pane closed without /exit）でも幽霊カウントを回避
  - [x] **prune.sh 拡張** — 既存「live でない pane」の削除条件を「live pane で claude を動かしている集合 にいない」に拡張。tmux 起動時 (`tmux.conf:15`) / switcher 起動時 (`switcher.sh:18`) / 任意の手動実行で死蔵 file を回収
- 残: **実機検証 (chezmoi apply 後に通す)**
  - [ ] `chezmoi diff` → `chezmoi apply` で `~/.local/bin/claude-cockpit-state.sh` / `~/.config/claude/settings.json` / `~/.config/tmux/scripts/cockpit/*` の差分を反映
  - [ ] `tmux source-file ~/.tmux.conf` で `bindings.conf` の `prefix + C + N` 説明文を再ロード
  - [ ] Claude を再起動して `settings.json` の `SessionEnd` hook entry がロードされたことを確認
  - [ ] **A 検証**: 任意の window で `/exit` → 数秒以内に status-right の `⏸ N` / `✓ N` のカウントが 1 減る
  - [ ] **B 検証**: claude pane を `prefix + C + k` 等で強制 kill (`/exit` を経由しない経路) → 直後の status-right 更新でカウントが 1 減る
  - [ ] **prune 検証**: 死蔵 status file を 1 つ手で作って `tmux kill-server` → 再起動後の `~/.cache/claude-cockpit/panes/` に残骸が無い
  - [ ] **next-ready 検証**: 複数 pane を意図的に `waiting` / `done` にして `prefix + C + N` で `waiting` 優先ジャンプを目視確認
- 残: **eBPF ベース process death リアルタイム検出 🚧 (検討中)**
  - 動機: B のフィルタは reader 呼出時にしか効かず、`prune.sh` も tmux 起動 / switcher 起動の bursty なタイミングのみ。`/exit` 以外の経路（SIGKILL / OOM / pane closed）では status file 削除が遅延する。kernel 側の終了イベントを直接観測すれば即時クリーンアップが可能。
  - アイデア:
    - `bpftrace` one-liner で `tracepoint:sched:sched_process_exit` に attach し、`comm == "claude"` の exit を捕捉
    - PID → tmux (session, pane_id) を `/proc/<pid>/environ` の `TMUX_PANE` 経由で解決（exec 時の env なので stale な可能性は低い）
    - 解決した key に対応する `panes/<S>_<P>.status` を即時 `rm -f`
    - bcc / cilium-ebpf 版でも同等。systemd --user で daemon 化候補
  - 懸念:
    - root 権限または `cap_sys_admin` / `cap_bpf` が必要 — `--user` unit で動かすには capability の取り回しを設計する必要
    - kernel バージョン依存 (Manjaro 想定なら 6.x で問題ないが、tracepoint vs kprobe の選択は要検証)
    - claude が node を fork した場合の親子区別 (terminal 内の claude プロセスのみが対象)
    - v1 の A + B + prune 拡張で実用上のカバレッジは十分高い見込みなので、痛みが顕在化してから着手する
  - 該当ファイル候補 (将来):
    - `dot_local/bin/executable_claude-cockpit-watch-bpf.sh` (新規)
    - `dot_config/systemd/user/claude-cockpit-watch.service` (新規、要 capability 設計)
    - `.chezmoiscripts/run_onchange_after_enable-claude-cockpit-watch.sh.tmpl` (bootstrap)
- 注意:
  - `pane_current_command == claude` の比較は live tmux の `display-message` 結果に依存。`claude` が `node` 等の interpreter 名で表示されるシェルラッパー経由の起動には未対応
  - `prune.sh` の cleanup 頻度は spec の "tmux 起動時 + switcher 起動時" 粒度のまま。eBPF が入るまではこの粒度で運用

---

## デファード（着手判断保留・小粒なフォローアップ）

過去レビューから派生したが、現状運用で痛みが小さいため後回しにしているもの。

- ~~**API キー lazy export 化**（C-1 派生）~~ → **完了 (2026-04-30)**: `private_secrets.zsh.tmpl` を `export` 廃止 → `__openrouter_key` (private getter) + `with_openrouter <cmd>` (単発注入ラッパー) に再構築。env から消えるため `/proc/<pid>/environ` 経由の観測リスクをさらに軽減。子プロセスへの自動継承は止まるため、ターミナル起動の VSCode / nvim / GUI 経由で OPENROUTER_API_KEY を期待するツールは `with_openrouter` を前置する運用に変更
- ~~**fast-syntax-highlighting の本家移行**（H-3 / H-12 派生）~~ → **完了 (2026-04-30)**: メンテ状況を gh API で確認した結果、auscompgeek フォークは 2020-02 で停止、zdharma-continuum は 2025-07 までアクティブ。`dot_config/sheldon/plugins.toml` を `zdharma-continuum/fast-syntax-highlighting` の v1.56 タグへ切替済み。次回 `sheldon lock` で新フォークが clone される
- ~~**mise `trusted_config_paths` の縮小**（H-4 派生）~~ → **完了 (2026-04-30)**: `~/programs` 全体から `data_manager` / `data_manager2` / `everything-claude-code` の 3 プロジェクト直下のみへ縮小。新規 mise プロジェクト追加時は明示登録する運用に変更
- ~~**`dircolor` alias の整理**（H-10 派生）~~ → **完了 (2026-04-30)**: `aliases.zsh:7` に用途コメントを追加 (alias 自体は dircolors 設定変更時の手動再読込ヘルパとして必要なので残置)
- ~~**chezmoi ソース dir 内の自己参照整理**（L-8 派生）~~ → **完了 (2026-04-30)**: `dot_local/share/chezmoi/dot_keep` を `git rm` で削除。`chezmoi managed` から `.local/share/chezmoi` の自己参照が消滅
- **`bw_lock` 自動化**（F-2 派生）: `precmd` フックで一定時間アイドルなら自動 `bw_lock`。過剰になりやすいため現時点では明示運用に留める。**2026-05-01 補足**: F-2 派生として tmpfs cache (`bw_session.zsh`) を導入したことでマスターパスワード再入力の頻度が大幅に減ったため、自動 `bw_lock` の優先度はさらに下がった。再起動で cache がクリアされる挙動と組み合わせて運用する

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
